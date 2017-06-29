package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

// App contains the functionality for the NGINX configurations
type App struct {
	ConfigDir        string             // The local directory to serialize configs into.
	TemplateFilePath string             // Path to the file containing the templatized NGINX config.
	Template         *template.Template // The parsed template
	Docker           *docker.Client     // The client used for Docker interactions.
	Matcher          *regexp.Regexp     // The regular expression for container names.
}

// InitApp sets up a new *App, returning an error if either the config directory
// or the template file does not exist.
func InitApp(configdir, templatefilepath string, dckr *docker.Client, m *regexp.Regexp) (*App, error) {
	d, err := os.Open(configdir)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %s", configdir)
	}
	defer d.Close()

	t, err := os.Open(templatefilepath)
	if err != nil {
		return nil, errors.Wrapf(err, "error opening %s", templatefilepath)
	}
	defer t.Close()

	tmpl, err := template.ParseFiles(templatefilepath)
	if err != nil {
		return nil, errors.Wrapf(err, "error parsing template %s", templatefilepath)
	}

	return &App{
		ConfigDir:        configdir,
		Docker:           dckr,
		Matcher:          m,
		TemplateFilePath: templatefilepath,
		Template:         tmpl,
	}, nil
}

// GenerateConfig uses the configured template and the change request to create
// a new nginx config.
func (a *App) GenerateConfig(c *ChangeRequest) ([]byte, error) {
	var err error
	buf := bytes.NewBuffer([]byte{})
	if err = a.Template.Execute(buf, c); err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

// ChangeRequest is the format for the requests sent to the service.
type ChangeRequest struct {
	Identifier string `json:"identifier"` // Only used for creating the config.
	URL        string `json:"url"`
	Host       string `json:"host"` // Parsed out of the URL if it's not set.
	Port       string `json:"port"` // Parse out of the URL if it's not set.
}

func parsebody(r *http.Request) (*ChangeRequest, error) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, errors.Wrap(err, "error reading request body")
	}
	defer r.Body.Close()
	c := &ChangeRequest{}
	err = json.Unmarshal(body, c)
	if err != nil {
		return nil, errors.Wrap(err, "error unmarshalling request body")
	}
	return c, nil
}

// InitChangeRequest will set any additional fields in the ChangeRequest based
// on the URL.
func (c *ChangeRequest) InitChangeRequest() error {
	if c.URL != "" {
		u, err := url.Parse(c.URL)
		if err != nil {
			return err
		}
		c.Host = u.Hostname()
		c.Port = u.Port()
	}
	return nil
}

// Add creates a configuration based on the incoming JSON.
func (a *App) Add(w http.ResponseWriter, r *http.Request) {
	// Parse the body of the request.
	change, err := parsebody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = change.InitChangeRequest(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate the configuration.
	cfg, err := a.GenerateConfig(change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.json", change.Identifier))
	configpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.conf", change.Identifier))
	if _, err = os.Stat(jsonpath); err == nil {
		err = fmt.Errorf("path exists already %s", jsonpath)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err = os.Stat(configpath); err == nil {
		err = fmt.Errorf("path exists already %s", configpath)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Serialize the JSON file to the filesystem.
	json, err := json.Marshal(change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = ioutil.WriteFile(jsonpath, json, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serialize the configuration file to the filesystem.
	if err = ioutil.WriteFile(configpath, cfg, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = a.SignalContainers(); err != nil {
		err = errors.Wrap(err, "error HUPing container(s)")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Update modifies a configuration based on the incoming JSON.
func (a *App) Update(w http.ResponseWriter, r *http.Request) {
	v := mux.Vars(r)
	identifier := v["identifier"]

	change, err := parsebody(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err = change.InitChangeRequest(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Generate the configuration.
	cfg, err := a.GenerateConfig(change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.json", identifier))
	configpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.conf", identifier))
	if _, err = os.Stat(jsonpath); os.IsNotExist(err) {
		err = errors.Wrapf(err, "path does not exist: %s", jsonpath)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if _, err = os.Stat(configpath); os.IsNotExist(err) {
		err = errors.Wrapf(err, "path does not exist: %s", configpath)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Serialize the JSON file to the filesystem.
	json, err := json.Marshal(change)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err = ioutil.WriteFile(jsonpath, json, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Serialize the configuration file to the filesystem.
	if err = ioutil.WriteFile(configpath, cfg, 0644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err = a.SignalContainers(); err != nil {
		err = errors.Wrap(err, "error HUPing container(s)")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Get returns the configuration based identifier parsed out of the URL.
func (a *App) Get(w http.ResponseWriter, r *http.Request) {
	var err error

	v := mux.Vars(r)
	identifier := v["identifier"]

	jsonpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.json", identifier))
	if _, err = os.Stat(jsonpath); os.IsNotExist(err) {
		err = errors.Wrapf(err, "path does not exist: %s", jsonpath)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	reader, err := os.Open(jsonpath)
	if err != nil {
		err = errors.Wrapf(err, "error opening path %s", jsonpath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	contents, err := ioutil.ReadAll(reader)
	if err != nil {
		err = errors.Wrapf(err, "error reading %s", jsonpath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = w.Write(contents); err != nil {
		err = errors.Wrapf(err, "error writing %s", jsonpath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Delete removes a configuration based on the identifier parsed out of the URL.
func (a *App) Delete(w http.ResponseWriter, r *http.Request) {
	var err error

	v := mux.Vars(r)
	identifier := v["identifier"]

	jsonpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.json", identifier))
	jsonpathexists := true

	configpath := path.Join(a.ConfigDir, fmt.Sprintf("%s.conf", identifier))
	configpathexists := true

	if _, err = os.Stat(jsonpath); os.IsNotExist(err) {
		jsonpathexists = false
	}
	if err != nil && !os.IsNotExist(err) {
		err = errors.Wrapf(err, "error checking file %s", jsonpath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, err = os.Stat(configpath); os.IsNotExist(err) {
		configpathexists = false
	}
	if err != nil && !os.IsNotExist(err) {
		err = errors.Wrapf(err, "error checking file %s", configpath)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if jsonpathexists {
		if err = os.Remove(jsonpath); err != nil {
			err = errors.Wrapf(err, "error removing file %s", jsonpath)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if configpathexists {
		if err = os.Remove(configpath); err != nil {
			err = errors.Wrapf(err, "error removing file %s", configpath)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if jsonpathexists || configpathexists {
		if err = a.SignalContainers(); err != nil {
			err = errors.Wrap(err, "error HUPing container(s)")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

// SignalContainers will send a HUP signal to the containers who have a name
// that match the regular expression.
func (a *App) SignalContainers() error {
	listopts := docker.ListContainersOptions{}
	containers, err := a.Docker.ListContainers(listopts)
	if err != nil {
		return errors.Wrap(err, "error listing containers")
	}

	for _, container := range containers {
		signal := false
		// Each container can have multiple names (apparently), so check them
		// all.
		for _, name := range container.Names {
			var n string
			fmt.Println(name)
			if strings.HasPrefix(name, "/") {
				n = strings.TrimPrefix(name, "/")
			} else {
				n = name
			}
			if a.Matcher.MatchString(n) {
				fmt.Println("matches")
				signal = true
			}
		}
		if signal {
			killopts := docker.KillContainerOptions{
				ID:     container.ID,
				Signal: docker.SIGHUP,
			}
			if err = a.Docker.KillContainer(killopts); err != nil {
				return err
			}
		}
	}
	return nil
}

func main() {
	var (
		nameRegex      = flag.String("regex", "", "The regex for the container names that should be signaled to reload configs.")
		dockerEndpoint = flag.String("docker-endpoint", "unix:///var/run/docker.sock", "The Docker URI.")
		templatePath   = flag.String("template", "default-template.tmpl", "The path to the nginx config template.")
		configDirPath  = flag.String("config-dir", "/etc/nginx/conf.d", "The path to the directory that will contain the nginx configs")
		listenAddr     = flag.String("listen-addr", "0.0.0.0:8080", "The listen port number.")
		sslCert        = flag.String("ssl-cert", "", "Path to the SSL .crt file.")
		sslKey         = flag.String("ssl-key", "", "Path to the SSL .key file.")
	)

	flag.Parse()

	useSSL := false
	if *sslCert != "" || *sslKey != "" {
		if *sslCert == "" {
			log.Fatal("--ssl-cert is required with --ssl-key.")
		}

		if *sslKey == "" {
			log.Fatal("--ssl-key is required with --ssl-cert.")
		}
		useSSL = true
	}

	nameMatcher := regexp.MustCompile(*nameRegex)

	client, err := docker.NewClient(*dockerEndpoint)
	if err != nil {
		log.Fatal(errors.Wrapf(err, "error connecting to Docker at %s", *dockerEndpoint))
	}

	app, err := InitApp(*configDirPath, *templatePath, client, nameMatcher)
	if err != nil {
		log.Fatal(errors.Wrap(err, "error initializing app"))
	}

	router := mux.NewRouter()
	api := router.PathPrefix("/api").Subrouter()
	api.Path("/{identifier}").Methods("GET").HandlerFunc(app.Get)
	api.Path("/").Methods("POST", "PUT").HandlerFunc(app.Add)
	api.Path("/{identifier}").Methods("POST", "PUT").HandlerFunc(app.Update)
	api.Path("/{identifier}").Methods("DELETE").HandlerFunc(app.Delete)

	server := &http.Server{
		Handler: router,
		Addr:    *listenAddr,
	}
	if useSSL {
		err = server.ListenAndServeTLS(*sslCert, *sslKey)
	} else {
		err = server.ListenAndServe()
	}
	log.Fatal(err)
}
