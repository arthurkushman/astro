package main

import (
	"fmt"
	"strings"
	"strconv"
	"github.com/mongodb/mongo-go-driver/mongo"
	"log"
	"context"
	"os/exec"
	"os"
)

type Entry struct {
	Id     string            `json:"id" bson:"_id,omitempty"`
	CityId int64             `json:"city_id" bson:"city_id"`
	Data   map[string]string `json:"data" bson:"data"`
}

type CommandArgs struct {
	Month    string
	Day      string
	From     string
	To       string
	EntryMap map[int64]*Entry
}

var entryMap = make(map[int64]*Entry)

func main() {
	tasks := make(chan []byte, 128)

	from := os.Args[1]
	to := os.Args[2]

	months := 12
	days := 31
	threadCounter := 0

	for m := 1; m <= months; m++ {
		month := fmt.Sprintf("%d", m)
		if m < 10 {
			month = fmt.Sprintf("%s%d", "0", m)
		}
		for d := 1; d <= days; d++ { // last days are dropped so don't care
			//wg.Add(1)
			day := fmt.Sprintf("%d", d)
			if d < 10 {
				day = fmt.Sprintf("%s%d", "0", d)
			}
			fmt.Println("month: ", month, "day: ", day)
			inputArgs := CommandArgs{Month: month, Day: day, From: from, To: to, EntryMap: entryMap}
			go func(inputArgs *CommandArgs, tasks chan []byte, m, d int) {
				args := []string{"weather:sun", inputArgs.Month, inputArgs.Day, inputArgs.From, inputArgs.To}
				cmd := exec.Command("/var/sites/gismeteo/current/console", args...)
				out, _ := cmd.Output()
				tasks <- out
			}(&inputArgs, tasks, m, d)
		}
	}

	// non-blocking selection of green-threads - awesome =) yum-yum
	for {
		select {
		case out := <-tasks:
			strSliced := strings.Split(string(out), "\n")
			// collect data like cityId, sunrise, sunset
			processOutput(strSliced)

			threadCounter++
			if threadCounter == months*days {
				fmt.Println("mongo connect...")
				collection := Connect()
				for _, e := range entryMap {
					e.Insert(collection)
				}
				fmt.Println("done...")
				close(tasks)
				os.Exit(0)
			}
			break
		default:
			// do nothing
		}
	}
}

func Connect() *mongo.Collection {
	client, err := mongo.NewClient("mongodb://mongo-dev01.gm:27017,mongo-dev02.gm:27017,mongo-dev03.gm:27017/?replicaSet=gis")
	if err != nil {
		log.Fatal(err)
	}
	errContext := client.Connect(context.Background())
	if errContext != nil {
		log.Fatal(errContext)
	}
	return client.Database("climate").Collection("gissun")
}

func processOutput(strSliced []string) {
	// starting process a day
	//fmt.Println("starting process a day")
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
func (entry *Entry) Insert(collection *mongo.Collection) {
	_, err := collection.InsertOne(context.Background(), entry)
	if err != nil {
		log.Fatal(err)
	}
	//fmt.Println(res.InsertedID)
}
