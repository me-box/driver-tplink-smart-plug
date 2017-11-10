package plugs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sausheong/hs1xxplug"
	databox "github.com/toshbrown/lib-go-databox"
)

var DATABOX_ZMQ_ENDPOINT = os.Getenv("DATABOX_ZMQ_ENDPOINT")

//Some timers and comms channels
var getDataChan = time.NewTicker(time.Second * 10).C
var scanForNewPlugsChan = time.NewTicker(time.Second * 600).C
var newPlugFoundChan = make(chan plug)

//default subnet to scan can be set by plugs.SetScanSubNet
var scan_sub_net = "192.168.1"

//A list of known plugs
var plugList = make(map[string]plug)

func PlugHandler() {

	tsc, err1 := databox.NewJSONTimeSeriesClient(DATABOX_ZMQ_ENDPOINT, false)
	if err1 != nil {
		fmt.Println("Error creating zest client", err1)
	}

	for {
		select {
		case <-getDataChan:
			fmt.Println("Updating plugs!! -> ", len(plugList))
			go updateReadings(tsc)
		case <-scanForNewPlugsChan:
			fmt.Println("Scanning for plugs!!")
			go scanForPlugs()
		case p := <-newPlugFoundChan:
			fmt.Println("New Plug Found!!")
			plugList[p.IP] = p
			go registerPlugWithDatabox(p, tsc)
		}
	}
}

func updateReadings(tsc databox.JSONTimeSeries_0_2_0) {

	resChan := make(chan *Reading)

	for _, p := range plugList {
		go func(pl plug, c chan<- *Reading) {
			fmt.Println("Getting data for ", pl.ID)
			data, err := getPlugData(pl.IP)
			if err == nil {
				c <- data
			} else {
				fmt.Println("Error getting data", err)
			}
		}(p, resChan)
	}

	for _, p := range plugList {
		res := <-resChan
		jsonString, err := json.Marshal(res.Emeter.GetRealtime)
		if err != nil {
			fmt.Println("Error unmarshing")
		}
		fmt.Println("Writing 1 Realtime::", p.ID, string(jsonString))
		err = tsc.Write(macToID(res.System.Mac), jsonString)
		if err != nil {
			fmt.Println("Error StoreJSONWriteTS", err)
		}

		jsonString, _ = json.Marshal(res.System.RelayState)
		err = tsc.Write("state-"+macToID(res.System.Mac), jsonString)
		if err != nil {
			fmt.Println("Error StoreJSONWriteTS", err)
		}
	}

	fmt.Println("Done Updating plugs!! -> ", len(plugList))

}

func SetPowerState(plugID string, state int) error {
	//find plug
	for ip, plug := range plugList {
		if plug.ID == plugID {
			p := hs1xxplug.Hs1xxPlug{IPAddress: ip}
			if state == 1 {
				p.TurnOn()
				return nil
			}
			p.TurnOff()
			return nil
		}
	}
	return errors.New("Plug " + plugID + " not found")
}

func scanForPlugs() {

	for i := 1; i < 255; i++ {

		go func(j int) {
			ip := scan_sub_net + "." + strconv.Itoa(j)
			url := "http://" + string(ip) + "/"

			if isPlugAtURL(url) == true && isPlugInList(ip) == false {
				fmt.Println("Plug found at", ip)
				res, _ := getPlugInfo(ip)
				p := plug{
					ID:    macToID(res.System.GetSysinfo.Mac),
					IP:    ip,
					Mac:   res.System.GetSysinfo.Mac,
					Name:  res.System.GetSysinfo.DevName,
					State: "Online",
				}
				newPlugFoundChan <- p
			}
		}(i)
	}

}

func registerPlugWithDatabox(p plug, tsc databox.JSONTimeSeries_0_2_0) {

	metadata := databox.DataSourceMetadata{
		Description:    "TP-Link Wi-Fi Smart Plug HS100 power usage",
		ContentType:    "application/json",
		Vendor:         "TP-Link",
		DataSourceType: "TP-Power-Usage",
		DataSourceID:   p.ID,
		StoreType:      "store-json",
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}

	tsc.RegisterDatasource(metadata)

	metadata = databox.DataSourceMetadata{
		Description:    "TP-Link Wi-Fi Smart Plug HS100 power state",
		ContentType:    "application/json",
		Vendor:         "TP-Link",
		DataSourceType: "TP-PowerState",
		DataSourceID:   "state-" + p.ID,
		StoreType:      "store-json",
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}
	tsc.RegisterDatasource(metadata)

	metadata = databox.DataSourceMetadata{
		Description:    "TP-Link Wi-Fi Smart Plug HS100 set power state",
		ContentType:    "application/json",
		Vendor:         "TP-Link",
		DataSourceType: "TP-SetPowerState",
		DataSourceID:   "setState-" + p.ID,
		StoreType:      "store-json",
		IsActuator:     true,
		Unit:           "",
		Location:       "",
	}
	tsc.RegisterDatasource(metadata)

	//subscribe for events on the setState actuator
	fmt.Println("Subscribing for update on ", "setState-"+p.ID)
	actuationChan, err := tsc.Observe("setState-" + p.ID)
	if err == nil {
		go func(actuationRequestChan <-chan []byte) {
			for {
				//blocks util request received
				request := <-actuationRequestChan
				fmt.Println("Got Actuation Request", string(request[:]))
				ar := actuationRequest{}
				err1 := json.Unmarshal(request, &ar)
				if err == nil {
					state := 1
					if ar.Data.Data == "off" {
						state = 0
					}
					err2 := SetPowerState(strings.Replace(ar.DatasourceID, "setState-", "", -1), state)
					if err2 != nil {
						fmt.Println("Error setting state ", err2)
					}
				} else {
					fmt.Println("Error parsing json ", err1)
				}
			}
		}(actuationChan)
	} else {
		fmt.Println("Error registering for Observe on " + "setState-" + p.ID)
	}

}

// SetScanSubNet is used to set the subnet to scan for new plugs
func SetScanSubNet(subnet string) {

	//TODO Validation

	scan_sub_net = subnet
}

// ForceScan will force a scan for new plugs
func ForceScan() {
	go scanForPlugs()
}

// GetPlugList returns a list of known plugs
func GetPlugList() map[string]plug {
	return plugList
}

func getPlugInfo(ip string) (*SysInfo, error) {
	p := hs1xxplug.Hs1xxPlug{IPAddress: ip}
	result, err := p.SystemInfo()
	if err != nil {
		return nil, err
	}
	j := new(SysInfo)
	jsonError := json.Unmarshal([]byte(result), j)
	return j, jsonError
}

func getPlugData(ip string) (*Reading, error) {
	p := hs1xxplug.Hs1xxPlug{IPAddress: ip}
	result, err := p.MeterInfo()
	if err != nil {
		return nil, err
	}
	j := new(Reading)
	jsonError := json.Unmarshal([]byte(result), j)
	return j, jsonError
}

func isPlugAtURL(url string) bool {
	client := &http.Client{Timeout: 1 * time.Second}
	req, _ := http.NewRequest("GET", url, nil)
	resp, err := client.Do(req)
	if err == nil {
		server := resp.Header.Get("Server")
		fmt.Println("Header::Server ", server)
		if server == "TP-LINK SmartPlug" {
			return true
		}
	}
	return false
}

func isPlugInList(ip string) bool {

	_, exists := plugList[ip]

	if exists {
		return true
	}

	return false
}

func macToID(mac string) string {
	return strings.Replace(mac, ":", "", -1)
}
