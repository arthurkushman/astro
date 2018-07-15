package main

import (
	"os/exec"
	"fmt"
	"strings"
	"strconv"
	"github.com/mongodb/mongo-go-driver/mongo"
	"log"
	"context"
)

type Entry struct {
	Id     string            `json:"id" bson:"_id,omitempty"`
	CityId int64             `json:"city_id" bson:"city_id"`
	Data   map[string]string `json:"data" bson:"data"`
}

var entryMap = make(map[int64]*Entry)

func main() {
	tasks := make(chan *exec.Cmd, 256)
	//var wg sync.WaitGroup
	for m := 1; m <= 12; m++ {
		month := string(m)
		if m < 10 {
			month = fmt.Sprintf("%s%d", "0", m)
		}
		for d := 1; d <= 31; d++ { // в случае перебора командная строка ничего не выведет - не заморачиваемся на даты
			day := string(d)
			if d < 10 {
				day = fmt.Sprintf("%s%d", "0", d)
			}
			go func(month string, tasks chan *exec.Cmd) {
				//defer wg.Done()
				args := []string{"weather:sun", month, day}
				tasks <- exec.Command("/var/sites/gismeteo/current/console", args...)
			}(month, tasks)
			cmd := <-tasks
			out, _ := cmd.Output()
			strSliced := strings.Split(string(out), "\n")
			// collect data like cityId, sunrise, sunset
			processOutput(strSliced)
		}
	}
	for _, e := range entryMap {
		//fmt.Println(e)
		e.Insert()
	}
	//wg.Wait()
}

func processOutput(strSliced []string) {
	var cityId int64 = 0
	var sunsetDt string = ""
	var sunriseDt string = ""
	for _, str := range strSliced {
		i := strings.Index(str, "|")
		if i > -1 {
			before := str[:i]
			dt := str[i+1:]
			if strings.Index("cityId", before) > -1 {
				cityId, _ = strconv.ParseInt(dt, 10, 64)
			}
			if strings.Index("Sunrise simple", before) > -1 {
				sunriseDt = dt
			}
			if strings.Index("Sunset simple ", before) > -1 {
				sunsetDt = dt
			}
			if cityId > 0 && sunriseDt != "" && sunsetDt != "" {
				if entryMap[cityId] == nil { // 1st time insert
					data := make(map[string]string)
					data[sunriseDt] = sunsetDt
					entry := &Entry{
						CityId: cityId,
						Data:   data,
					}
					entryMap[cityId] = entry
				} else { // update
					// find by city_id
					data := entryMap[cityId].Data
					// append sunset/sunrise via RAM
					data[sunriseDt] = sunsetDt
					entryMap[cityId] = &Entry{
						CityId: cityId,
						Data:   data,
					}
				}
				cityId = 0
				sunriseDt = ""
				sunsetDt = ""
			}
		}
	}
}

// write object to mongodb
func (entry *Entry) Insert() {
	client, err := mongo.NewClient("mongodb://localhost:27017")
	if err != nil {
		log.Fatal(err)
	}
	err = client.Connect(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	collection := client.Database("climate").Collection("gissun")
	res, err := collection.InsertOne(context.Background(), entry)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println(res.InsertedID)
}
