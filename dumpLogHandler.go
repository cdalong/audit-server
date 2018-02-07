package main

import (
	"net/http"
	"os"
	"encoding/xml"
	"reflect"
	"database/sql"
	"strconv"
	"fmt"
	"io"
	"encoding/json"
)

func (s logEntry) MarshalXML( e *xml.Encoder, start xml.StartElement) error {
	start.Name.Local = s["logType"]
	delete(s, "logType")
	err := e.EncodeToken(start)
	if err != nil { failGracefully(err, "could not encode token ") }
	for k, v := range s { e.Encode(xmlMapEntry{XMLName: xml.Name{Local: k}, Value: v}) }
	return e.EncodeToken(start.End())
}

func structToMap(i interface{}) (values logEntry){
	values = make(logEntry)
	iVal := reflect.ValueOf(i).Elem()
	typ := iVal.Type()
	for i := 0; i <iVal.NumField(); i++ {
		f := iVal.Field(i)
		tag := typ.Field(i).Tag.Get("paramName")
		var v string

		switch f.Interface().(type) {
		case sql.NullString:
			if f.Field(1).Bool(){
				v = f.Field(0).String()
			} else {
				continue
			}
		case sql.NullInt64:
			if f.Field(1).Bool(){
				v = strconv.FormatInt(f.Field(0).Int(), 10)
			} else {
				continue
			}
		case sql.NullFloat64:
			if f.Field(1).Bool(){
				v = strconv.FormatFloat(f.Field(0).Float(), 'f', 2, 64)
			} else {
				continue
			}
		default:
			v = fmt.Sprint(f.Interface())
		}

		values[tag] = v
	}
	return values
}

func writeToXML(w io.Writer, r logDB) {
	var output []byte
	var err error

	rMap := structToMap(&r)
	output, err = xml.MarshalIndent(rMap, "    ", "    ")
	if err != nil { failGracefully(err, "Failed to write to file ") }

	w.Write(output)
	w.Write([]byte("\n"))
}

func dumpLogHandler(w http.ResponseWriter, r *http.Request) {
	logDumpCommand(w, r)
	f, err := os.Create("log.xml")
	if err != nil { failGracefully(err, "Failed to open log file ") }

	rows, err := db.Query("SELECT logType, (extract(EPOCH FROM timestamp) * 1000)::BIGINT as timestamp, server, transactionNum, command, username, stockSymbol, filename, (funds::DECIMAL)/100 as funds, cryptokey, (price::DECIMAL)/100 as price, quoteServerTime, action, errorMessage, debugMessage FROM audit_log;")
	if err != nil { failGracefully(err, "Failed to query audit DB ") }
	defer rows.Close()
	f.Write([]byte("<?xml version=\"1.0\"?>\n"))
	f.Write([]byte("<log>\n"))
	for rows.Next() {
		var l logDB
		err = rows.Scan(&l.LogType, &l.Timestamp, &l.Server, &l.TransactionNum, &l.Command, &l.Username, &l.StockSymbol, &l.Filename, &l.Funds, &l.Cryptokey, &l.Price, &l.QuoteServerTime, &l.Action, &l.ErrorMessage, &l.DebugMessage)
		writeToXML(f, l)
	}
	f.Write([]byte("</log>\n"))
	f.Close()
}

func dumpLogUserHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	res := struct{
		Username        string  `json:"username"`
		TransactionNum  int     `json:"transactionNum"`
		Filename        string  `json:"filename"`
		Server          string  `json:"server"`
	}{"", -1, FILENAME, SERVER}
	err := decoder.Decode(&res)
	if err != nil {
		failWithStatusCode(err, http.StatusText(http.StatusBadRequest), w, http.StatusBadRequest)
		return
	}

	queryString := "INSERT INTO audit_log(timestamp, transactionnum, server, command, filename, logtype) VALUES (now(), $1, $2, $3, $4, 'userCommand')"
	stmt, err := db.Prepare(queryString)

	resdb, err := stmt.Exec(res.TransactionNum, res.Server, "DUMPLOG", res.Filename)

	checkErrors(resdb, err, w)

	f, err := os.Create("log"+ res.Username + ".xml")
	if err != nil { failGracefully(err, "Failed to open log file ") }

	queryString = "SELECT logType, (extract(EPOCH FROM timestamp) * 1000)::BIGINT as timestamp, server, transactionNum, command, username, stockSymbol, filename, (funds::DECIMAL)/100 as funds, cryptokey, (price::DECIMAL)/100 as price, quoteServerTime, action, errorMessage, debugMessage FROM audit_log WHERE username = $1;"
	stmt, err = db.Prepare(queryString)

	rows, err := stmt.Query(res.Username)
	if err != nil { failGracefully(err, "Failed to query audit DB ") }
	defer rows.Close()

	f.Write([]byte("<?xml version=\"1.0\"?>\n"))
	f.Write([]byte("<log>\n"))
	for rows.Next() {
		var l logDB
		err = rows.Scan(&l.LogType, &l.Timestamp, &l.Server, &l.TransactionNum, &l.Command, &l.Username, &l.StockSymbol, &l.Filename, &l.Funds, &l.Cryptokey, &l.Price, &l.QuoteServerTime, &l.Action, &l.ErrorMessage, &l.DebugMessage)
		writeToXML(f, l)
	}
	f.Write([]byte("</log>\n"))
	f.Close()
}

func logDumpCommand(w http.ResponseWriter, r *http.Request){

	decoder := json.NewDecoder(r.Body)
	res := struct{
		Username        string  `json:"username"`
		TransactionNum  int     `json:"transactionNum"`
		Filename        string  `json:"filename"`
		Server          string  `json:"server"`
	}{"", -1, FILENAME, SERVER}
	err := decoder.Decode(&res)
	if err != nil {
		failWithStatusCode(err, http.StatusText(http.StatusBadRequest), w, http.StatusBadRequest)
		return
	}

	queryString := "INSERT INTO audit_log(timestamp, transactionnum, server, command, filename, logtype) VALUES (now(), $1, $2, $3, $4, 'userCommand')"
	stmt, err := db.Prepare(queryString)

	resdb, err := stmt.Exec(res.TransactionNum, res.Server, "DUMPLOG", res.Filename)

	checkErrors(resdb, err, w)
}
