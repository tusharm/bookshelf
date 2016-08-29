package bookshelf

import (
	"fmt"
	"log"
	"strconv"

	"golang.org/x/net/context"

	"google.golang.org/appengine/search"
)

const BooksIndex = "books"

type Doc struct {
	Title string
}

func IndexBook(ctxt context.Context, book *Book) error {
	index, err := search.Open(BooksIndex)
	if err != nil {
		return err
	}
	_, err = index.Put(ctxt, fmt.Sprintf("%v", book.ID), &Doc{book.Title})
	return err
}

func SearchBooks(ctxt context.Context, term string) ([]int64, error) {
	index, err := search.Open(BooksIndex)
	if err != nil {
		return nil, err
	}

	ids := make([]int64, 0)
	for t := index.Search(ctxt, term, nil); ; {
		var doc Doc
		idString, err := t.Next(&doc)
		if err == search.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		id, _ := strconv.Atoi(idString)
		ids = append(ids, int64(id))
	}

	log.Printf("Search with term [%v] found [%v] books", term, len(ids))
	return ids, nil
}

func DeleteBook(ctxt context.Context, id int64) error {
	index, err := search.Open(BooksIndex)
	if err != nil {
		return err
	}

	return index.Delete(ctxt, fmt.Sprintf("%v", id))
}