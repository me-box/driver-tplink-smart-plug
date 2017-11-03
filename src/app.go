package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

	"./plugs"

	"github.com/gorilla/mux"
	databox "github.com/toshbrown/lib-go-databox"
)

var dataStoreHref = os.Getenv("DATABOX_STORE_ENDPOINT")

func getStatusEndpoint(w http.ResponseWriter, req *http.Request) {
	w.Write([]byte("active\n"))
}

func displayUI(w http.ResponseWriter, req *http.Request) {
	var templates *template.Template
	templates, err := template.ParseFiles("tmpl/settings.tmpl")
	if err != nil {
		fmt.Println(err)
		w.Write([]byte("error\n"))
		return
	}
	s1 := templates.Lookup("settings.tmpl")
	err = s1.Execute(w, plugs.GetPlugList())
	if err != nil {
		fmt.Println(err)
		w.Write([]byte("error\n"))
		return
	}
}

func scanForPlugs(w http.ResponseWriter, req *http.Request) {

	req.ParseForm()
	if val := req.FormValue("plugSubNet"); val != "" {

		fmt.Println("scanning subnet ", val)

		plugs.SetScanSubNet(val)
		go plugs.ForceScan()

		return
	}

	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
	return

}

type data struct {
	Data string `json:"data"`
}

type actuationRequest struct {
	DatasourceID string `json:"datasource_id"`
	Data         data   `json:"data"`
	Timestamp    int64  `json:"timestamp"`
	ID           string `json:"_id"`
}

var DATABOX_ZMQ_ENDPOINT = os.Getenv("DATABOX_ZMQ_ENDPOINT")

func main() {

	fmt.Println("DATABOX_ZMQ_ENDPOINT", DATABOX_ZMQ_ENDPOINT)
	kvc, err := databox.NewKeyValueClient(DATABOX_ZMQ_ENDPOINT, true)
	if err != nil {
		fmt.Println("Error creating zest client", err)
	}

	metadata := databox.StoreMetadata{
		Description:    "Test data source",
		ContentType:    "text/json",
		Vendor:         "tosh",
		DataSourceType: "toshTest",
		DataSourceID:   "tosh",
		StoreType:      "store-core",
		IsActuator:     false,
		Unit:           "Kg",
		Location:       "",
	}
	kvc.RegisterDatasource("toshTest", metadata)

	payloadChan, obsErr := kvc.Observe("toshTest")
	if obsErr != nil {
		fmt.Println("Error setting state:: ", obsErr)
	} else {
		go func(payload <-chan string) {
			for {
				fmt.Println("Waiting for data .........")
				fmt.Println("Observe Got Data:: ", <-payload)
				fmt.Println("Done waiting  for data .........")
			}
		}(payloadChan)
	}

	writeErr := kvc.Write("toshTest", "{\"hello\":\"world\"}")
	if writeErr != nil {
		fmt.Println("Error setting state ", writeErr)
	}

	kvc.Write("toshTest", "{\"hello\":\"world2\"}")

	kvc.Write("toshTest", "{\"hello\":\"world3\"}")

	//data, readErr := kvc.Read("tosh")
	//if readErr != nil {
	//	fmt.Println("Error setting state ", readErr)
	//}
	//fmt.Println("Got Data:: ", data)

	tsc, err1 := databox.NewTimeSeriesClient(DATABOX_ZMQ_ENDPOINT, false)
	if err1 != nil {
		fmt.Println("Error creating zest client", err1)
	}

	metadata = databox.StoreMetadata{
		Description:    "Test data source",
		ContentType:    "text/json",
		Vendor:         "tosh",
		DataSourceType: "toshTest",
		DataSourceID:   "tosh",
		StoreType:      "store-core",
		IsActuator:     false,
		Unit:           "Kg",
		Location:       "",
	}
	tsc.RegisterDatasource("toshTest", metadata)

	writeErr1 := tsc.Write("tosh", "{\"hello\":\"ts world\"}")
	if writeErr1 != nil {
		fmt.Println("Error setting state ", writeErr1)
	}

	data1, readErr1 := tsc.Latest("tosh")
	if readErr1 != nil {
		fmt.Println("Error setting state ", readErr1)
	}
	fmt.Println("Got Data:: ", data1)

	writeErr2 := tsc.WriteAt("tosh", (time.Now().UnixNano()+10000)/1000000, "{\"hello\":\"ts world 2\"}")
	if writeErr2 != nil {
		fmt.Println("Error setting state ", writeErr2)
	}

	data2, readErr2 := tsc.Latest("tosh")
	if readErr2 != nil {
		fmt.Println("Error setting state ", readErr2)
	}
	fmt.Println("Got Data:: ", data2)

	//Wait for my store to become active
	//databox.WaitForStoreStatus(dataStoreHref)

	//start the plug handler it scans for new plugs and polls for data
	go plugs.PlugHandler()

	go plugs.ForceScan()

	//actuation
	/*actuationChan, err := databox.WSConnect(dataStoreHref)
	if err == nil {

		go func(actuationRequestChan chan []byte) {
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
					err2 := plugs.SetPowerState(strings.Replace(ar.DatasourceID, "setState-", "", -1), state)
					if err2 != nil {
						fmt.Println("Error setting state ", err2)
					}
				} else {
					fmt.Println("Error parsing json ", err1)
				}
			}
		}(actuationChan)

	} else {
		fmt.Println("Error connecting to websocket for actuation", err)
	}*/

	//
	// Handel Https requests
	//
	router := mux.NewRouter()

	router.HandleFunc("/status", getStatusEndpoint).Methods("GET")
	router.HandleFunc("/ui", displayUI).Methods("GET")
	router.HandleFunc("/ui", scanForPlugs).Methods("POST")

	static := http.StripPrefix("/ui/static", http.FileServer(http.Dir("./www/")))
	router.PathPrefix("/ui/static").Handler(static)

	log.Fatal(http.ListenAndServeTLS(":8080", databox.GetHttpsCredentials(), databox.GetHttpsCredentials(), router))
}
