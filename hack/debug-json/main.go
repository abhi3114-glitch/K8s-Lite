package main

import (
	"encoding/json"
	"fmt"

	"github.com/abhigod/k8s-lite/internal/api"
)

func main() {
	jsonStr := `{"targetPort": 80}`
	type T struct {
		TargetPort api.IntOrString `json:"targetPort"`
	}
	var t T
	err := json.Unmarshal([]byte(jsonStr), &t)
	if err != nil {
		fmt.Printf("Error 80: %v\n", err)
	} else {
		fmt.Printf("Success 80: %+v\n", t)
	}

	jsonStr2 := `{"targetPort": "http"}`
	var t2 T
	err = json.Unmarshal([]byte(jsonStr2), &t2)
	if err != nil {
		fmt.Printf("Error string: %v\n", err)
	} else {
		fmt.Printf("Success string: %+v\n", t2)
	}
}





