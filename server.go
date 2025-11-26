package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	serial "go.bug.st/serial"
)

//go:embed index.html
var content embed.FS

type MachineStatus string

const (
	Off            MachineStatus = "off"
	Running        MachineStatus = "running"
	Malfunctioning MachineStatus = "malfunctioning"
)

type Data struct {
	Amperage  float64       `json:"amperage"`
	Vibration bool          `json:"vibration"`
	Status    MachineStatus `json:"status"`
	Timestamp string        `json:"timestamp"`
}

type SensorData struct {
	Vibration bool
	Amperage  float64
}

var (
	currentData SensorData
	dataMutex   sync.RWMutex
	runningMin  float64
	malfMin     float64
)

func main() {
	// Configuración del puerto serial
	port := getSerialPort()
	
	// Configuración de red
	ip := getIP()
	portNum := getPort()

	// Configuración de rangos de amperaje
	getAmperageRanges()

	// Iniciar lectura del puerto serial
	go readSerialData(port)

	// Configurar rutas HTTP
	http.HandleFunc("/", serveIndex)
	http.HandleFunc("/data", serveData)

	// Iniciar servidor
	addr := fmt.Sprintf("%s:%d", ip, portNum)
	log.Printf("Servidor iniciado en http://%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

func getSerialPort() string {
	ports, err := serial.GetPortsList()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Puertos seriales disponibles:")
	for i, port := range ports {
		fmt.Printf("%d: %s\n", i+1, port)
	}

	fmt.Print("Selecciona el puerto (enter para autodetectar): ")
	var input string
	fmt.Scanln(&input)

	if input == "" {
		if len(ports) > 0 {
			return ports[0]
		}
		log.Fatal("No se detectaron puertos seriales")
	}

	var index int
	if _, err := fmt.Sscanf(input, "%d", &index); err != nil || index < 1 || index > len(ports) {
		log.Fatal("Selección inválida")
	}

	return ports[index-1]
}

func getIP() string {
	fmt.Print("IP del servidor (enter para host expose): ")
	var ip string
	fmt.Scanln(&ip)

	if ip == "" {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			log.Fatal(err)
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
		log.Fatal("No se pudo detectar la IP")
	}

	return ip
}

func getPort() int {
	fmt.Print("Puerto HTTP (enter para 9090): ")
	var port string
	fmt.Scanln(&port)

	if port == "" {
		return 9090
	}

	var portNum int
	if _, err := fmt.Sscanf(port, "%d", &portNum); err != nil {
		log.Fatal("Puerto inválido")
	}

	return portNum
}

func getAmperageRanges() {
	fmt.Println("Configuración de rangos de amperaje:")
	
	fmt.Print("Amperaje mínimo para RUNNING: ")
	_, err := fmt.Scanln(&runningMin)
	if err != nil {
		log.Fatal("Valor inválido")
	}

	fmt.Print("Amperaje mínimo para MALFUNCTIONING: ")
	_, err = fmt.Scanln(&malfMin)
	if err != nil {
		log.Fatal("Valor inválido")
	}

	if runningMin >= malfMin {
		log.Fatal("El amperaje mínimo para RUNNING debe ser menor que para MALFUNCTIONING")
	}
}

func readSerialData(portName string) {
	mode := &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}

	port, err := serial.Open(portName, mode)
	if err != nil {
		log.Fatal(err)
	}
	defer port.Close()

	buf := make([]byte, 128)
	var buffer string

	for {
		n, err := port.Read(buf)
		if err != nil {
			log.Printf("Error leyendo puerto serial: %v", err)
			continue
		}

		buffer += string(buf[:n])

		lines := strings.Split(buffer, "\n")
		buffer = lines[len(lines)-1]

		for _, line := range lines[:len(lines)-1] {
			processLine(strings.TrimSpace(line))
		}
	}
}

func processLine(line string) {
	if strings.HasPrefix(line, "#") || line == "" {
		return
	}

	fields := strings.Fields(line)
	if len(fields) != 2 {
		log.Printf("Línea malformada: %s", line)
		return
	}

	var vibration bool
	switch fields[0] {
	case "Y":
		vibration = true
	case "N":
		vibration = false
	default:
		log.Printf("Valor de vibración inválido: %s", fields[0])
		return
	}

	var amperage float64
	if _, err := fmt.Sscanf(fields[1], "%f", &amperage); err != nil {
		log.Printf("Amperaje inválido: %s", fields[1])
		return
	}

	dataMutex.Lock()
	currentData = SensorData{
		Vibration: vibration,
		Amperage:  amperage,
	}
	dataMutex.Unlock()

	//log.Printf("Datos actualizados - Vibración: %t, Amperaje: %.2f", vibration, amperage)
}

func determineStatus(amperage float64) MachineStatus {
	switch {
	case amperage >= malfMin:
		return Malfunctioning
	case amperage >= runningMin:
		return Running
	default:
		return Off
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	html, err := content.ReadFile("index.html")
	if err != nil {
		http.Error(w, "Error interno del servidor", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
}

func serveData(w http.ResponseWriter, r *http.Request) {
	dataMutex.RLock()
	sensorData := currentData
	dataMutex.RUnlock()

	response := Data{
		Amperage:  sensorData.Amperage,
		Vibration: sensorData.Vibration,
		Status:    determineStatus(sensorData.Amperage),
		Timestamp: time.Now().Format("15:04:05"),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
