package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

type MoneyTransfer struct {
	Id      int
	Name    string
	Balance int
}

func dbConn() (db *sql.DB, err error) {
	dbDriver := "mysql"
	dbUser := "root"
	dbPass := ""
	dbName := "moneyTransfer"

	db, err = sql.Open(dbDriver, dbUser+":"+dbPass+"@/"+dbName)
	return
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func homePage(w *loggingResponseWriter, r *http.Request) {
	pageDir := "pages/"
	fileName := "index.html"
	t, err := template.ParseFiles(pageDir + fileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	if err = t.ExecuteTemplate(w, fileName, nil); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func getUsers(w *loggingResponseWriter, r *http.Request) {
	db, err := dbConn()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	defer db.Close()

	rows, err := db.Query("SELECT * FROM MoneyTransfer ORDER BY id ASC")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}

	users := []MoneyTransfer{}

	for rows.Next() {
		var id, balance int
		var name string

		//Assigns the values inside vars
		err = rows.Scan(&id, &name, &balance)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			return
		}

		users = append(users, MoneyTransfer{
			Id:      id,
			Name:    name,
			Balance: balance,
		})
	}

	pageDir := "pages/"
	fileName := "getUsers.html"
	t, err := template.ParseFiles(pageDir + fileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	if err = t.ExecuteTemplate(w, fileName, users); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func makePayment(w *loggingResponseWriter, r *http.Request) {
	sendToName := r.FormValue("sendToNameHidden")
	// sendToBalance := r.FormValue("sendToBalanceHidden")
	sendFromName := r.FormValue("sendFromName")
	sendAmount, _ := strconv.Atoi(r.FormValue("sendAmount"))

	var msg string
	var isErr bool

	if sendToName == sendFromName {
		msg += "Self-transfer not allowed!\n"
		isErr = true
	}

	if sendAmount <= 0 {
		msg += "Enter non-zero positive amount\n"
		isErr = true
	}

	if !isErr {
		db, err := dbConn()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer db.Close()

		rows, err := db.Query("SELECT * FROM MoneyTransfer WHERE name=?", sendFromName)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			return
		}

		for rows.Next() {
			var id, balance int
			var name string
			err = rows.Scan(&id, &name, &balance)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Println(err)
				return
			}

			if balance < sendAmount {
				msg += "Insufficient funds in " + name + "'s account\n"
				isErr = true
				goto paymentGoto
			}
		}

		updateSender, err := db.Prepare("UPDATE MoneyTransfer SET balance=balance-? WHERE name=?")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer updateSender.Close()
		updateSender.Exec(sendAmount, sendFromName)

		updateReceiver, err := db.Prepare("UPDATE MoneyTransfer SET balance=balance+? WHERE name=?")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Println(err)
			return
		}
		defer updateReceiver.Close()
		updateReceiver.Exec(sendAmount, sendToName)

		msg += "Transaction successful"
	}

paymentGoto:
	pageDir := "pages/"
	fileName := "makePayment.html"
	t, err := template.ParseFiles(pageDir + fileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
	type PaymentStatus struct {
		Message string
		IsErr   bool
	}
	if err = t.ExecuteTemplate(w, fileName, PaymentStatus{Message: msg, IsErr: isErr}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(err)
		return
	}
}

func router(w http.ResponseWriter, r *http.Request) {
	lrw := NewLoggingResponseWriter(w)
	defer func() {
		log.Println(r.Method, "--", r.URL.Path, "--", lrw.statusCode)
	}()
	switch r.URL.Path {
	case "/":
		homePage(lrw, r)
	case "/users":
		getUsers(lrw, r)
	case "/payment":
		makePayment(lrw, r)
	default:
		http.Redirect(lrw, r, "/", http.StatusMovedPermanently)
	}
}

func main() {
	http.Handle("/pages/", http.StripPrefix("/pages", http.FileServer(http.Dir("./pages"))))
	http.HandleFunc("/", router)

	log.Println("Listening at port 1234...")
	if err := http.ListenAndServe(":1234", nil); err != nil {
		log.Fatal(err)
	}
}
