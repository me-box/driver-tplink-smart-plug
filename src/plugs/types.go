package plugs

type plug struct {
	ID    string
	IP    string
	Mac   string
	Name  string
	State string
}

type GetSysinfo struct {
	ErrCode    int     `json:"err_code"`
	SwVer      string  `json:"sw_ver"`
	HwVer      string  `json:"hw_ver"`
	Type       string  `json:"type"`
	Model      string  `json:"model"`
	Mac        string  `json:"mac"`
	DeviceID   string  `json:"deviceId"`
	HwID       string  `json:"hwId"`
	FwID       string  `json:"fwId"`
	OemID      string  `json:"oemId"`
	Alias      string  `json:"alias"`
	DevName    string  `json:"dev_name"`
	IconHash   string  `json:"icon_hash"`
	RelayState int     `json:"relay_state"`
	OnTime     int     `json:"on_time"`
	ActiveMode string  `json:"active_mode"`
	Feature    string  `json:"feature"`
	Updating   int     `json:"updating"`
	Rssi       int     `json:"rssi"`
	LedOff     int     `json:"led_off"`
	Latitude   float64 `json:"latitude"`
	Longitude  float64 `json:"longitude"`
}

type SysInfo struct {
	System struct {
		GetSysinfo `json:"get_sysinfo"`
	} `json:"system"`
}

type Reading struct {
	System struct {
		GetSysinfo `json:"get_sysinfo"`
	} `json:"system"`
	Emeter struct {
		GetRealtime struct {
			Current float64 `json:"current"`
			Voltage float64 `json:"voltage"`
			Power   float64 `json:"power"`
			Total   float64 `json:"total"`
			ErrCode int     `json:"err_code"`
		} `json:"get_realtime"`
		GetVgainIgain struct {
			Vgain   float64 `json:"vgain"`
			Igain   float64 `json:"igain"`
			ErrCode int     `json:"err_code"`
		} `json:"get_vgain_igain"`
	} `json:"emeter"`
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
