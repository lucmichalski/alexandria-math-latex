// Alexandria
//
// Copyright (C) 2015,2016  Colin Benner
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/pprof"

	flag "github.com/ogier/pflag"

	. "github.com/yzhs/alexandria"
)

// Generate a HTML file describing the size of the library.
func printStats() string {
	stats := ComputeStatistics()
	n := stats.Num()
	size := float32(stats.Size()) / 1024.0
	return fmt.Sprintf("The library contains %v scrolls with a total size of %.1f kiB.\n", n, size)
}

// Send the statistics page to the client.
func statsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, printStats())
}

// Handle the edit-link, causing the browser to open that scroll in an editor.
func editHandler(w http.ResponseWriter, r *http.Request) {
	headers := w.Header()
	headers["Content-Type"] = []string{"application/x-alexandria-edit"}
	id := r.FormValue("id")
	fmt.Fprintf(w, id)
}

// Serve the search page.
func mainHandler(w http.ResponseWriter, r *http.Request) {
	html, err := loadHtmlTemplate("main")
	if err != nil {
		fmt.Fprintf(w, "%v", err)
		return
	}
	fmt.Fprintln(w, string(html))
}

func loadHtmlTemplate(name string) ([]byte, error) {
	return ioutil.ReadFile(Config.TemplateDirectory + name + ".html")
}

type result struct {
	Query        string
	Matches      []Id
	NumMatches   int
	TotalMatches int
}

func renderTemplate(w http.ResponseWriter, templateFile string, resultData result) {
	t, err := template.ParseFiles(Config.TemplateDirectory + templateFile + ".html")
	if err != nil {
		fmt.Fprintf(w, "Error: %v", err)
		return
	}
	t.Execute(w, resultData)
}

func min(a, b int) int {
	if a > b {
		return b
	}
	return a
}

// Handle a query and serve the results.
func queryHandler(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	if query == "" {
		mainHandler(w, r)
		return
	}
	results, err := FindScrolls(query)
	if err != nil {
		panic(err)
	}
	numMatches := len(results.Ids)
	data := result{Query: query, NumMatches: numMatches, Matches: results.Ids[:min(20, numMatches)],
		TotalMatches: results.Total}
	renderTemplate(w, "search", data)
}

func serveDirectory(prefix string, directory string) {
	http.Handle(prefix, http.StripPrefix(prefix, http.FileServer(http.Dir(directory))))
}

func main() {
	var profile, version bool
	flag.BoolVarP(&version, "version", "v", false, "\tShow version")
	flag.BoolVar(&profile, "profile", false, "\tEnable profiler")
	flag.Parse()

	InitConfig()
	Config.MaxResults = 20

	if profile {
		f, err := os.Create("alexandria.prof")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// TODO run GenerateIndex() when there is something new

	if version {
		fmt.Println(NAME, VERSION)
		return
	}

	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/search", queryHandler)
	http.HandleFunc("/alexandria.edit", editHandler)
	serveDirectory("/images/", Config.CacheDirectory)
	serveDirectory("/styles/", Config.TemplateDirectory+"styles")
	http.ListenAndServe("127.0.0.1:41665", nil)
}
