package urlshortner

import (
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"github.com/gorilla/mux"
	"github.com/mauricioklein/go-url-shortner/store"
)

// Router defines the routes definition
// for the project
type Router struct {
	ls *store.LinkStore
	rs *store.RedisStore
	mr *mux.Router
}

func NewRouter(ls *store.LinkStore, rs *store.RedisStore) *mux.Router {
	mr := mux.NewRouter()
	router := Router{
		ls: ls,
		rs: rs,
		mr: mr,
	}

	mr.HandleFunc("/{code}", router.proxyURLCode).Methods(http.MethodGet)
	mr.HandleFunc("/", router.homepage).Methods(http.MethodGet)
	mr.HandleFunc("/", router.registerURL).Methods(http.MethodPost)

	return mr
}

func (router *Router) homepage(w http.ResponseWriter, r *http.Request) {
	t, err := template.ParseFiles("templates/home.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	t.Execute(w, nil)
}

func (router *Router) registerURL(w http.ResponseWriter, r *http.Request) {
	url, err := getUrlFromForm(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Load template
	t, err := template.ParseFiles("templates/response.html")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// check if link already exists on database
	storedLink, err := router.ls.GetByURL(url)
	if err == nil {
		t.Execute(w, buildShortURL(r.Host, storedLink))
		return
	}

	// It's a new link, so let's persist it
	newLink, err := router.ls.PersistURL(url)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	t.Execute(w, buildShortURL(r.Host, newLink))
}

func (router *Router) proxyURLCode(w http.ResponseWriter, r *http.Request) {
	code, exists := mux.Vars(r)["code"]
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := decode(code)

	// first check if the link exists in Redis
	redisLink, err := router.rs.QueryLinkByID(id)
	if err == nil {
		http.Redirect(w, r, redisLink, http.StatusMovedPermanently)
		return
	}

	// link not found on Redis.
	// So, let's query the DB
	dbLink, err := router.ls.GetByID(id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// store the link on Redis
	router.rs.StoreLink(dbLink)

	http.Redirect(w, r, dbLink.URL, http.StatusMovedPermanently)
}

func getUrlFromForm(r *http.Request) (string, error) {
	if err := r.ParseForm(); err != nil {
		return "", err
	}

	urls := r.Form["url"]

	if len(urls) == 0 {
		return "", errors.New("no url provided")
	}

	// check URL validity
	if _, err := url.ParseRequestURI(urls[0]); err != nil {
		return "", err
	}

	return urls[0], nil
}

func buildShortURL(host string, link *store.Link) string {
	code := encode(link.ID)
	return fmt.Sprintf("http://%s/%s", host, code)
}
