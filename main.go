package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/MrDoctorKovacic/MDroid-Core/logging"
	"github.com/MrDoctorKovacic/MDroid-Core/settings"
	drok "github.com/MrDoctorKovacic/drok"
	"github.com/tarm/serial"
)

// MainStatus will control logging and reporting of status / warnings / errors
var MainStatus = logging.NewStatus("Main")

// Start with program arguments
var (
	settingsFile string
	drokAddress  string
	drokPort     *serial.Port
	mdroidHost   string
)

func main() {

	// Repeatedly run config, to ensure serial device is grabbed
	cok := config()
	for !cok {
		cok = config()
	}

	for {
		voltage, vok := readValue("voltage")
		if vok {
			//fmt.Println(voltage)
			postValue(fmt.Sprintf("%f", voltage), "AUX_VOLTAGE_OUTPUT")
		}

		current, cok := readValue("current")
		if cok {
			// Increase voltage at load
			if vok {
				if current >= 2 && voltage == 5.24 {
					drok.SetVoltage(drokPort, 5.35)
				} else if current < 2 && voltage == 5.35 {
					drok.SetVoltage(drokPort, 5.24)
				}
			}

			postValue(fmt.Sprintf("%f", current), "AUX_CURRENT")
		}

		time.Sleep(time.Second * 10)
	}
}

func readValue(valueName string) (float32, bool) {
	switch valueName {
	case "current":
		// Read output current
		current, err := drok.ReadCurrent(drokPort)
		if err != nil {
			MainStatus.Log(logging.Error(), fmt.Sprintf("Error reading current: \n%s", err.Error()))
		}
		return current, true
	case "voltage":
		// Read output voltage
		voltage, err := drok.ReadVoltage(drokPort)
		if err != nil {
			MainStatus.Log(logging.Error(), fmt.Sprintf("Error reading voltage: \n%s", err.Error()))
		}
		return voltage, true
	}

	return 0, false
}

func postValue(value string, valueType string) {
	jsonStr := []byte(fmt.Sprintf(`{"value":"%s"}`, value))
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/session/%s", mdroidHost, valueType), bytes.NewBuffer(jsonStr))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		MainStatus.Log(logging.Error(), err.Error())
		return
	}
	MainStatus.Log(logging.OK(), fmt.Sprintf("Successfully posted %s to %s", valueType, mdroidHost))
	defer resp.Body.Close()
}

func config() bool {
	flag.StringVar(&settingsFile, "settings-file", "", "File to recover the persistent settings.")
	flag.Parse()

	// Parse settings file
	settingsData, _ := settings.ReadFile(settingsFile)

	// Log settings
	out, err := json.Marshal(settingsData)
	if err != nil {
		MainStatus.Log(logging.Error(), err.Error())
		return false
	}
	MainStatus.Log(logging.OK(), "Using settings: "+string(out))

	// Parse through config if found in settings file
	config, ok := settingsData["MDROID"]
	if ok {
		// Set up drok
		drokAddressString, usingdrok := config["DROK_DEVICE"]
		if !usingdrok {
			MainStatus.Log(logging.Error(), "Drok address not found in settings file")
			return false
		}
		drokAddress = drokAddressString

		// Set up drok
		mdroidAddress, usingMDroid := config["MDROID_HOST"]
		if !usingMDroid {
			MainStatus.Log(logging.Error(), "MDroid address not found in settings file")
			return false
		}
		mdroidHost = mdroidAddress
	} else {
		MainStatus.Log(logging.Error(), "No config found in settings file, not parsing through config")
	}

	c := &serial.Config{Name: drokAddress, Baud: 4800}
	drokPort, err = serial.OpenPort(c)
	if err != nil {
		MainStatus.Log(logging.Error(), "No config found in settings file, not parsing through config")
		time.Sleep(time.Second * 10)
		MainStatus.Log(logging.Warning(), "Trying to config again...")
		return false
	}
	return true
}
