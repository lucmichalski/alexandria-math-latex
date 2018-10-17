// This file is part of Alexandria which is released under AGPLv3.
// Copyright (C) 2015-2018 Colin Benner
// See LICENSE or go to https://github.com/yzhs/alexandria/LICENSE for full
// license details.

package alexandria

import (
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/blevesearch/bleve/analysis/analyzer/keyword"
	"github.com/blevesearch/bleve/analysis/analyzer/simple"
	"github.com/pkg/errors"
)

// UpdateIndex adds all documents to the index that have been created or
// modified since the last time this function was executed.
//
// Note that this function does *not* remove deleted documents from the index.
// See `RemoveFromIndex`.
func updateIndex() error {
	index, isNewIndex, err := openOrCreateIndex()
	if err != nil {
		return errors.Wrap(err, "open or create index")
	}
	defer index.Close()

	indexUpdateFile := Config.AlexandriaDirectory + "index_updated"
	timeOfLastIndexUpdate, err := getModTime(indexUpdateFile)
	// If an error occurs, we just log it. In that case,
	// timeOfLastIndexUpdate will contain 0, i.e. 1970-01-01. The entire
	// purpose of the `index_updated` file is to reduce the number of
	// documents we reindex. Therefore, the worst case scenario when
	// getModTime fails is that we do some redundant work.
	tryLogError(err)
	recordIndexUpdateStart(indexUpdateFile)

	files, err := ioutil.ReadDir(Config.KnowledgeDirectory)
	if err != nil {
		return errors.Wrap(err, "read knowledge directory")
	}

	batch := index.NewBatch()
	for _, file := range files {
		if !isNewIndex && isOlderThan(file, timeOfLastIndexUpdate) {
			continue
		}

		id := strings.TrimSuffix(file.Name(), ".tex")
		scroll, err := loadAndParseScrollContentByID(ID(id))
		if err != nil {
			logError(err)
			continue
		}
		err = batch.Index(id, scroll)
		if err != nil {
			logError(err)
		}
	}
	return index.Batch(batch)
}

func recordIndexUpdateStart(indexUpdateFile string) {
	err := touch(indexUpdateFile)
	tryLogError(err)
}

func touch(file string) error {
	now := time.Now()
	return os.Chtimes(file, now, now)
}

func openOrCreateIndex() (bleve.Index, bool, error) {
	isNewIndex := false

	index, err := openExistingIndex()
	if err != nil {
		index, err = createNewIndex()
		isNewIndex = true
	}

	return index, isNewIndex, err
}

func openExistingIndex() (bleve.Index, error) {
	return bleve.Open(Config.AlexandriaDirectory + "bleve")
}

func createNewIndex() (bleve.Index, error) {
	enTextMapping := bleve.NewTextFieldMapping()
	enTextMapping.Analyzer = "en"

	simpleMapping := bleve.NewTextFieldMapping()
	simpleMapping.Analyzer = simple.Name

	typeMapping := bleve.NewTextFieldMapping()
	typeMapping.Analyzer = keyword.Name

	scrollMapping := bleve.NewDocumentMapping()
	scrollMapping.AddFieldMappingsAt("id", simpleMapping)
	scrollMapping.AddFieldMappingsAt("content", enTextMapping)
	scrollMapping.AddFieldMappingsAt("type", typeMapping)
	scrollMapping.AddFieldMappingsAt("source", enTextMapping)
	scrollMapping.AddFieldMappingsAt("tag", enTextMapping)
	scrollMapping.AddFieldMappingsAt("hidden", enTextMapping)
	scrollMapping.AddFieldMappingsAt("other", enTextMapping)

	mapping := bleve.NewIndexMapping()
	mapping.DefaultAnalyzer = "en"
	mapping.DefaultMapping = scrollMapping

	return bleve.New(Config.AlexandriaDirectory+"bleve", mapping)
}

func isOlderThan(file os.FileInfo, indexUpdateTime int64) bool {
	modTime, err := getModTime(Config.KnowledgeDirectory + file.Name())
	if err != nil {
		logError(err)
		return true
	}
	return modTime < indexUpdateTime
}

func loadAndParseScrollContentByID(id ID) (Scroll, error) {
	content, err := readScroll(id)
	if err != nil {
		return Scroll{}, err
	}
	return parse(string(id), content), nil
}

// RemoveFromIndex removes a specified document from the index. This is
// necessary as UpdateIndex has no way of knowing if a document was deleted.
func removeFromIndex(id ID) error {
	index, err := openExistingIndex()
	if err != nil {
		return err
	}
	defer index.Close()
	return index.Delete(string(id))
}

func translatePlusMinusTildePrefixes(queryString string) string {
	newQueryString := ""
	for _, tmp := range strings.Split(queryString, " ") {
		word := strings.TrimSpace(tmp)
		if word[0] == '-' || word[0] == '+' {
			newQueryString += " " + word
		} else if word[0] == '~' {
			// Remove prefix to make term optional
			newQueryString += " " + word[1:]
		} else {
			newQueryString += " +" + word
		}
	}
	return newQueryString[1:] // Remove leading space
}

func performQuery(index bleve.Index, newQueryString string) (*bleve.SearchResult, error) {
	query := bleve.NewQueryStringQuery(newQueryString)
	search := bleve.NewSearchRequest(query)
	search.Size = Config.MaxResults
	return index.Search(search)
}

// computeStatistics counts the number of scrolls in the library and computes
// their combined size.
func computeStatistics() (Statistics, error) {
	index, err := openExistingIndex()
	if err != nil {
		return Stats{}, errors.Wrap(err, "open existing index")
	}
	defer index.Close()

	_, size, err := getDirSize(Config.KnowledgeDirectory)
	if err != nil {
		return Stats{}, errors.Wrap(err, "get size of library directory")
	}

	num, err := index.DocCount()
	if err != nil {
		return Stats{}, errors.Wrap(err, "get number of scrolls in the index")
	}

	return Stats{int(num), size}, nil
}
