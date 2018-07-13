package main

import (
	"os/exec"
	"fmt"
	"time"
)

func main() {
	start, _ := time.Parse("2019-1-1", "2019-12-31")
	for d := start; d.Month() == start.Month(); d = d.AddDate(0, 0, 1) {
		сmd := exec.Command("./gissun -y " + string(d.Year()) + " -m " + string(d.Month()) + " -d "+ string(d.Day()) + " -tz 180 -lat  -lon ")

		out, err := сmd.Output()
		if err != nil {
			panic(err)
		}
		fmt.Println(string(out))
	}
}
