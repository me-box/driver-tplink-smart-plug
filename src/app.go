package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"text/template"

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

	/*tsc, err1 := databox.NewJSONTimeSeriesClient(DATABOX_ZMQ_ENDPOINT, true)
	if err1 != nil {
		fmt.Println("Error creating zest client", err1)
	}*/

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
