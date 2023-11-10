package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/gorilla/handlers"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"time"
)

type mysql_binlog_status struct {
	File     string
	Position float64
}

type canal_binlog_status struct {
	ClientDatas []ClientData `json:"clientDatas"`
}

type ClientData struct {
	Cursor struct {
		Postion struct {
			JournalName string  `json:"journalName"`
			Position    float64 `json:"position"`
		} `json:"postion"`
	} `json:"cursor"`
}

var (
	mysql_binlog = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mysql_binlog",
		Help: "mysql_binlog",
	}, []string{"name", "file"})

	canal_binlog = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "canal_binlog",
		Help: "canal_binlog",
	}, []string{"name", "file"})
	result   mysql_binlog_status
	username = os.Getenv("username")
	password = os.Getenv("password")
	host     = os.Getenv("host")
	filepath = os.Getenv("filepath")
	health   bool
)

func mysqlStatus() {
	dsn := fmt.Sprintf("%v:%v@tcp(%v:%v)/%v?charset=utf8mb4&parseTime=True&loc=Local", username, password, host, 3306, "mysql")
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		health = false
		log.Panic(err.Error())
	}
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn: sqlDB,
	}), &gorm.Config{})
	if err != nil {
		health = false
		log.Panic(err.Error())
	}

	go func() {
		for {
			time.Sleep(5 * time.Second)
			mysql_binlog.Reset()
			db.Raw("show master status").Scan(&result)
			mysql_binlog.WithLabelValues("mysql", result.File).Set(result.Position)
		}
	}()
}

func canalStatus() {
	go func() {
		for {
			time.Sleep(5 * time.Second)
			canal_binlog.Reset()
			content, err := os.ReadFile(filepath)
			if err != nil {
				health = false
				log.Println(err.Error())
				continue
			}
			var result canal_binlog_status
			if err := json.Unmarshal(content, &result); err != nil {
				health = false
				log.Println(err.Error())
				continue
			}
			canal_binlog.WithLabelValues("canal", result.ClientDatas[0].Cursor.Postion.JournalName).Set(result.ClientDatas[0].Cursor.Postion.Position)
		}
	}()
}

func init() {
	log.SetFlags(log.LstdFlags)
}

func main() {
	health = true
	mysqlStatus()
	canalStatus()
	http.HandleFunc("/health", func(writer http.ResponseWriter, request *http.Request) {
		if health {
			writer.WriteHeader(200)
			writer.Write([]byte("ok"))
		} else {
			writer.WriteHeader(500)
			writer.Write([]byte("failure"))
		}
	})
	http.Handle("/metrics", promhttp.Handler())
	if err := http.ListenAndServe(":9120", handlers.LoggingHandler(os.Stdout, http.DefaultServeMux)); err != nil {
		log.Fatal(err)
	}
}
