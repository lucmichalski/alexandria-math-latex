// Alexandria
//
// Copyright (C) 2015  Colin Benner
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"runtime/pprof"
	"strings"

	flag "github.com/ogier/pflag"

	. "github.com/yzhs/alexandria-go"
	render "github.com/yzhs/alexandria-go/render/xelatex"
)

func printStats() string {
	stats := render.ComputeStatistics()
	n := stats.Num()
	size := float32(stats.Size()) / 1024.0
	return fmt.Sprintf("The library contains %v scrolls with a total size of %.1f kiB.\n", n, size)
}

func statsHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, printStats())
}

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
	Query      string
	Matches    []Id
	NumMatches int
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

func queryHandler(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	ids, err := render.FindScrolls(strings.Split(query, " "))
	if err != nil {
		panic(err)
	}
	data := result{Query: query, NumMatches: len(ids), Matches: ids[:min(20, len(ids))]}
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

	// TODO run when there is something new render.GenerateIndex()

	if version {
		fmt.Println(NAME, VERSION)
		return
	}

	http.HandleFunc("/", mainHandler)
	http.HandleFunc("/stats", statsHandler)
	http.HandleFunc("/search", queryHandler)
	serveDirectory("/images/", Config.CacheDirectory)
	serveDirectory("/styles/", Config.TemplateDirectory+"styles")
	http.ListenAndServe("127.0.0.1:8080", nil)
}