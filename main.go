package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jinzhu/gorm"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"net/smtp"
	"os"
	"regexp"
	"strconv"
	"time"
)

type Handler struct {
	DB   *gorm.DB
	Tmpl *template.Template
}

const emailPattern = "^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$"

func main() {
	handlers := &Handler{
		DB:   GetDBClient(),
		Tmpl: template.Must(template.ParseGlob("./tmpl/*")),
	}
	defer handlers.DB.Close()

	r := mux.NewRouter()

	r.HandleFunc("/", handlers.index).Methods("GET")
	r.HandleFunc("/create", handlers.createGroup).Methods("GET").Queries("count", "{count:[0-9]+}")
	r.HandleFunc("/{link}", handlers.email).Methods("GET")
	r.HandleFunc("/{link}", handlers.join).Methods("POST")
	http.ListenAndServe(":8080", r)
}

func (h *Handler) join(w http.ResponseWriter, req *http.Request) {
	fmt.Println("join call")
	vars := mux.Vars(req)
	link := vars["link"]
	var group Group
	h.DB.Table("groups").Select("*").Where("link = ?", link).Scan(&group)
	if group.ID != 0 {
		email := req.FormValue("email")
		name := req.FormValue("name")
		fmt.Printf("link:%v email: %v name: %v \n", link, email, name)
		if !regexp.MustCompile(emailPattern).MatchString(email) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var person Person
		if err := h.DB.Where("email = ?", email).First(&person).Error; err != nil {
			person := Person{Email:email, Name:name}
			h.DB.Save(&person)
		}
		h.DB.Model(&person).Where("email = ?", email).Update("name", name)
		h.DB.Where("email = ?", email).First(&person)
		h.DB.Save(&GroupToPerson{GroupID: group.ID, PersonID: person.ID})
		fmt.Printf("Added relation: groupId: %v, PersonId: %v", group.ID, person.ID)
		var count uint
		h.DB.Table("group_to_people").Where("group_id = ?", group.ID).Count(&count)
		fmt.Printf("Joined to group %v: %v", group.ID, count)
		if count == group.PersonsCount {
			defer func() {
				h.DB.Model(&group).Where("link = ?", group.Link).Update("closed", true)
				fmt.Println(group.ID)
				var persons []Person
				h.DB.Table("people").Select("people.id, people.name, people.email").Joins(
					"inner join group_to_people on people." +
						"id = group_to_people.person_id").Where("group_to_people.group_id = ?", group.ID).Scan(&persons)
				fmt.Println(persons)
				sendEmails(persons)
			}()
			h.Tmpl.ExecuteTemplate(w, "last.html", nil)
			return
		}
		h.Tmpl.ExecuteTemplate(w, "success.html", nil)
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func getPairs(persons []Person) map[Person]Person {
	rand.Seed(time.Now().UnixNano())
	rand.Shuffle(len(persons), func(i, j int) { persons[i], persons[j] = persons[j], persons[i] })
	m := make(map[Person]Person, len(persons))
	m[persons[len(persons)-1]] = persons[0]
	for i := 0; i < len(persons)-1; i++ {
		m[persons[i]] = persons[i+1]
	}
	return m
}

func sendEmails(persons []Person) {
	pairs := getPairs(persons)
	for i := range pairs {
		send(i, pairs[i])
	}
}

func (h *Handler) email(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	link := vars["link"]
	var group Group
	if err := h.DB.Where("link = ?", link).First(&group).Error; err == nil {
		if group.Closed {
			h.Tmpl.ExecuteTemplate(w, "closed.html", nil)
			return
		}
		var count uint
		fmt.Println(group.ID)
		h.DB.Table("group_to_people").Where("group_id = ?", group.ID).Count(&count)
		type placeHolder struct {
			Placeholder string
		}
		h.Tmpl.ExecuteTemplate(w, "join.html", &placeHolder{Placeholder: fmt.Sprintf("%v/%v joined", count,
			group.PersonsCount)})
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (h *Handler) index(w http.ResponseWriter, req *http.Request) {
	h.Tmpl.ExecuteTemplate(w, "index.html", nil)
}

func (h *Handler) createGroup(w http.ResponseWriter, req *http.Request) {
	count, _ := strconv.Atoi(req.FormValue("count"))
	link := getLink()
	h.DB.Save(&Group{PersonsCount: uint(count), Link: link})
	type tmpStruct struct {
		Link string
	}
	h.Tmpl.ExecuteTemplate(w, "link.html", &tmpStruct{os.Getenv("SANTA_ADDRESS") + link})
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func getLink() string {
	b := make([]byte, 16)
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	for i := range b {
		b[i] = letterBytes[r1.Intn(len(letterBytes))]
	}
	return string(b)
}

func send(giver, receiver Person) {
	from := os.Getenv("MAIL_ADDRESS")
	pass := os.Getenv("MAIL_PASS")

	msg := "From: " + from + "\n" +
		"To: " + giver.Email + "\n" +
		"Subject: Secret santa\n\n" +
		"Hello, " + giver.Name +"! Your receiver is " + receiver.Name + "\n" +
		"You can contact him via email: " + receiver.Email + "\n" +
		"Ho-ho-ho!"

	err := smtp.SendMail("smtp.gmail.com:587",
		smtp.PlainAuth("", from, pass, "smtp.gmail.com"),
		from, []string{giver.Email}, []byte(msg))

	if err != nil {
		log.Printf("smtp error: %s", err)
		return
	}
}
