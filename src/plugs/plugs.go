package plugs

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	databox "github.com/me-box/lib-go-databox"
	"github.com/cgreenhalgh/hs1xxplug"
)

var DATABOX_ZMQ_ENDPOINT = os.Getenv("DATABOX_ZMQ_ENDPOINT")

//Some timers and comms channels
var getDataChan = time.NewTicker(time.Second * 10).C
var scanForNewPlugsChan = time.NewTicker(time.Second * 600).C
var newPlugFoundChan = make(chan plug)

//default subnet to scan can be set by plugs.SetScanSubNet
var scan_sub_net = "192.168.0"

//A list of known plugs
var plugList = make(map[string]plug)

var tsc = databox.NewDefaultCoreStoreClient(DATABOX_ZMQ_ENDPOINT)

func PlugHandler() {

	tsc := databox.NewDefaultCoreStoreClient(DATABOX_ZMQ_ENDPOINT)
	ReadSettings()

	for {
		select {
		case <-getDataChan:
			fmt.Println("Updating plugs!! -> ", len(plugList))
			go updateReadings(tsc)
		case <-scanForNewPlugsChan:
			fmt.Println("Scanning for plugs!!")
			go scanForPlugs()
		case p := <-newPlugFoundChan:
			if ( !isPlugInList( p.IP ) ) {
				fmt.Println("New Plug Found!!")
				plugList[p.IP] = p
				go registerPlugWithDatabox(p, tsc)
			}
		}
	}
}

func updateReadings(tsc *databox.CoreStoreClient) {

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
		err = tsc.TSBlobJSON.Write(macToID(res.System.Mac), jsonString)
		if err != nil {
			fmt.Println("Error StoreJSONWriteTS", err)
		}

		jsonString, _ = json.Marshal(res.System.RelayState)
		saveString := `{"state":` + string(jsonString) + `}`
		fmt.Println("Writing 2 Realtime::", p.ID, saveString)
		err = tsc.TSBlobJSON.Write("state-"+macToID(res.System.Mac), []byte(saveString))
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
			fmt.Println("Scanning ", ip)
			if isPlugAtIP(ip) == true && isPlugInList(ip) == false {
				fmt.Println("Plug found at", ip)
				res, _ := getPlugInfo(ip)
				fmt.Println(res)
				var name string
				if res.System.GetSysinfo.Alias != "" {
					name = res.System.GetSysinfo.Alias
				} else {
					name = res.System.GetSysinfo.DevName
				}
				p := plug{
					ID:    macToID(res.System.GetSysinfo.Mac),
					IP:    ip,
					Mac:   res.System.GetSysinfo.Mac,
					Name:  name,
					State: "Online",
				}
				newPlugFoundChan <- p
			}
		}(i)
	}

}

func registerPlugWithDatabox(p plug, tsc *databox.CoreStoreClient) {

	metadata := databox.DataSourceMetadata{
		Description:    fmt.Sprintf("TP-Link Wi-Fi Smart Plug HS100 '%s' (%s) power usage", p.Name, p.ID),
		ContentType:    "application/json",
		Vendor:         "TP-Link",
		DataSourceType: "TP-Power-Usage",
		DataSourceID:   p.ID,
		StoreType:      databox.StoreTypeTSBlob,
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}

	tsc.RegisterDatasource(metadata)

	metadata = databox.DataSourceMetadata{
		Description:    fmt.Sprintf("TP-Link Wi-Fi Smart Plug HS100 '%s' (%s) power state", p.Name, p.ID),
		ContentType:    "application/json",
		Vendor:         "TP-Link",
		DataSourceType: "TP-PowerState",
		DataSourceID:   "state-" + p.ID,
		StoreType:      databox.StoreTypeTSBlob,
		IsActuator:     false,
		Unit:           "",
		Location:       "",
	}
	tsc.RegisterDatasource(metadata)

	metadata = databox.DataSourceMetadata{
		Description:     fmt.Sprintf("TP-Link Wi-Fi Smart Plug HS100 '%s' (%s) set power state", p.Name, p.ID),
		ContentType:    "application/json",
		Vendor:         "TP-Link",
		DataSourceType: "TP-SetPowerState",
		DataSourceID:   "setState-" + p.ID,
		StoreType:      databox.StoreTypeTSBlob,
		IsActuator:     true,
		Unit:           "",
		Location:       "",
	}
	tsc.RegisterDatasource(metadata)

	//subscribe for events on the setState actuator
	fmt.Println("Subscribing for update on ", "setState-"+p.ID)
	actuationChan, err := tsc.TSBlobJSON.Observe("setState-" + p.ID)
	if err == nil {
		go func(actuationRequestChan <-chan databox.ObserveResponse) {
			for {
				fmt.Println("Waiting for request on ", "setState-"+p.ID)
				//blocks util request received
				request := <-actuationRequestChan
				fmt.Println("Got Actuation Request", string(request.Data[:]), " on ", "setState-"+p.ID)
				ar := actuationRequest{}
				err1 := json.Unmarshal(request.Data, &ar)
				if err == nil {
					state := 1
					if ar.Data == "off" {
						state = 0
					}
					err2 := SetPowerState(p.ID, state)
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
	writeSettings()
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

func isPlugAtIP(ip string) bool {
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, err := d.Dial("tcp", ip+":9999")
	if err != nil {
		fmt.Println("[isPlugAtIP] Error ", err)
		return false
	}
	conn.Close()
	return true
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

const SETTINGS_DATASOURCEID = "TPLinkSettings"
const SETTINGS_KEY = "settings"
type Settings struct {
	ScanSubNet string `json: "scan_sub_net"`
}
func GetSettings() Settings {
	return Settings{
		ScanSubNet: scan_sub_net,
	}
}

func ReadSettings() {
	var settings Settings
	payload, err := tsc.KVJSON.Read(SETTINGS_DATASOURCEID, SETTINGS_KEY)
	if err != nil {
		fmt.Println("Error reading settings: "+err.Error())
		return
	}
	err = json.Unmarshal(payload, &settings)
	if err != nil {
		fmt.Println("Error unmarshalling settings: "+err.Error())
		return
	}
	if len( settings.ScanSubNet ) >0 {
		scan_sub_net = settings.ScanSubNet
		fmt.Println("Restore scan_sub_net to ", scan_sub_net)
	}
}

func writeSettings() {
	settings := Settings{
		ScanSubNet:  scan_sub_net,
	}
	jsonData, err :=  json.Marshal(settings)
	if err != nil {
		fmt.Println("Error marshalling settings: "+err.Error())
		return
	}	
	err = tsc.KVJSON.Write(SETTINGS_DATASOURCEID, SETTINGS_KEY, []byte(jsonData))
	if err != nil {
		fmt.Println("Error writing settings: "+err.Error())
	}
	fmt.Println("Wrote settings")
}
