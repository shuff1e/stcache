package main

import (
	"encoding/json"
	"errors"
	"fmt"
	gorilla "github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Detector is a failure detector
type Detector struct {
	w          map[string]*Windowed
	windowSize int
	minSamples int
	mu         sync.Mutex
	Logger *log.Logger
}

// New returns a new failure detector that considers the last windowSize
// samples, and ensures there are at least minSamples in the window before
// returning an answer
func NewDetector(windowSize, minSamples int) *Detector {

	d := &Detector{
		windowSize: windowSize,
		minSamples: minSamples,
		w: make(map[string]*Windowed,0),
	}

	return d
}

func (d *Detector) Clone() map[string][]StatusAndTime {
	tempDetectorData := map[string][]StatusAndTime{}
	d.mu.Lock()
	defer d.mu.Unlock()
	for k,v := range d.w {
		tempDetectorData[k] = v.data
	}
	return tempDetectorData
}

func (d *Detector) Get(key string) ([]StatusAndTime,error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if v,ok := d.w[key];ok {
		return v.data,nil
	}
	return []StatusAndTime{},errors.New(fmt.Sprintf("key %s not found",key))
}

func (d *Detector) DelKey(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.w,key)
	return nil
}

// Ping registers a heart-beat at time now
func (d *Detector) Ping(verificationScript string, input verifyInput) {
	key := input.key
	value := input.value
	cmd := fmt.Sprintf(verificationFormat,verificationScript,key,value)
	stdout,stderr,err := ShellCmdTimeout(verificationScriptTimeoiut,"sh","-c",cmd)
	stderr = strings.TrimSuffix(stderr,"\n")
	if stderr != "" {
		d.Logger.Println(cmd,"stderr:",stderr)
	}
	if err != nil {
		d.Logger.Println(cmd,"err:",err)
	}
	stdout = strings.TrimSuffix(stdout,"\n")
	if stdout == "" {
		stdout = "EMPTY"
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _,ok := d.w[key];!ok {
		d.w[key] = NewWindowed(d.windowSize)
	}
	d.w[key].Push(StatusAndTime{stdout,time.Now()})
}

// Phi calculates the suspicion level at time 'now' that the remote end has failed
func (d *Detector) Phi(key string) gossipStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.w[key].Len() < d.minSamples {
		return GOSSIP_LESS_THAN_MINSAMPLE
	}

	countMap := make(map[gossipStatus]int)
	for _,v := range d.w[key].LastNSamples(d.minSamples) {
		countMap[gossipStatus(v.Status)] += 1
	}

	for status,count := range countMap {
		if count == d.minSamples {
			return status
		}
	}

	if countMap[GOSSIP_EMPTY] == 0 {
		return GOSSIP_NO_EMPTY_IN_SAMPLES
	}

	return GOSSIP_NOT_FILL_SAMPLES
}

//
type Windowed struct {
	data []StatusAndTime
	head int

	length int
}

type StatusAndTime struct {
	Status string
	Time time.Time
}

func NewWindowed(capacity int) *Windowed {
	return &Windowed{
		data: make([]StatusAndTime, capacity),
	}
}

func (w *Windowed) Push(n StatusAndTime) StatusAndTime {
	old := w.data[w.head]

	w.length++

	w.data[w.head] = n
	w.head++
	if w.head >= len(w.data) {
		w.head = 0
	}
	return old
}

func (w *Windowed) Len() int {
	if w.length < len(w.data) {
		return w.length
	}

	return len(w.data)
}

// samples <= Len()
func (w *Windowed) LastNSamples(samples int) []StatusAndTime {
	ret := make([]StatusAndTime,samples)
	length := w.Len()
	for i := 0; i < samples; i++ {
		ret[samples-1-i] = w.data[(w.head - 1 - i + length)%length]
	}
	return ret
}

// Set sets the current windowed data.  The length is reset, and Push() will start overwriting the first element of the array.
func (w *Windowed) Set(data []StatusAndTime) {
	if w.data != nil {
		w.data = w.data[:0]
	}

	w.data = append(w.data, data...)

	w.head = 0
	w.length = len(data)
}

func (d *Detector) doDetectorData(w http.ResponseWriter, r *http.Request) {
	tempDetectorData := d.Clone()
	//fmt.Println(tempDetectorData)
	dataBytes, err := json.Marshal(tempDetectorData)
	if err != nil {
		d.Logger.Printf("doDetectorData failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	io.WriteString(w,string(dataBytes))
}

func (d *Detector) doDetectorDataForKey(w http.ResponseWriter, r *http.Request) {
	vars := gorilla.Vars(r)
	key := vars["key"]
	tempDetectorData,err := d.Get(key)
	if err != nil {
		d.Logger.Printf("doDetectorDataForKey failed, err:%v", err)
		fmt.Fprint(w, err)
		return
	}
	//fmt.Println(tempDetectorData)
	dataBytes, err := json.Marshal(map[string][]StatusAndTime{key:tempDetectorData})
	if err != nil {
		d.Logger.Printf("doDetectorDataForKey failed, err:%v", err)
		fmt.Fprint(w, "internal error\n")
		return
	}
	io.WriteString(w,string(dataBytes))
}
