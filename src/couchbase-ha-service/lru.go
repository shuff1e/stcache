package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type node struct {
	key string
	value string
	prev *node
	next *node
}

type Cache struct {
	mp map[string]*node
	mutex sync.Mutex
	cap int
	head *node
	tail *node
}

func NewLRUCache(cap int) *Cache {
	return &Cache{make(map[string]*node), sync.Mutex{}, cap, nil, nil}
}

func (c *Cache) Get(key string) (value string,ok bool) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	var node *node
	if node,ok = c.mp[key];ok {
		value = node.value
		c.removeAndInsert(node)
	}
	return
}

func (c *Cache) removeAndInsert(node *node) {
	// remove
	if node == c.head {
	} else if node == c.tail {
		c.tail = node.prev
		c.tail.next = nil
	} else {
		node.prev.next = node.next
		node.next.prev = node.prev
		node.prev = nil
		node.next = nil
	}
	// insert at head
	node.next = c.head
	c.head.prev = node
	c.head = node
}

func (c *Cache) Put(key,value string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.head == nil {
		c.head = &node{key:key, value:value}
		c.tail = c.head
		c.mp[key] = c.head
		return
	}
	n,ok := c.mp[key]
	if ok {
		n.value = value
		c.removeAndInsert(n)
		return
	}
	n = &node{key:key , value:value}
	if len(c.mp) >= c.cap {
		c.delTail()
	}
	c.mp[key] = n
	n.next = c.head
	c.head.prev = n
	c.head = n
}

func (c *Cache) delTail() {
	delete(c.mp,c.tail.key)
	c.tail = c.tail.prev
	c.tail.next = nil
}

func ArticlesCategoryHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Category: %v\n", vars["key"])
}

func test() {
	r := mux.NewRouter()
	r.HandleFunc("/products/{key}", ArticlesCategoryHandler)
	srv := &http.Server{
		Handler:      r,
		Addr:         "127.0.0.1:8000",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	log.Fatal(srv.ListenAndServe())

	cache := NewLRUCache(3)
	wg := sync.WaitGroup{}
	for i :=0; i < 1; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			cache.Put(strconv.Itoa(i),strconv.Itoa(i))
			fmt.Println(cache.Get(strconv.Itoa(i+2)))
			cache.Put(strconv.Itoa(i+1),strconv.Itoa(i+1))
			cache.Put(strconv.Itoa(i+2),strconv.Itoa(i+2))
			fmt.Println(cache.Get(strconv.Itoa(i+2)))
			cache.Put(strconv.Itoa(i+3),strconv.Itoa(i+3))
			fmt.Println(cache.Get(strconv.Itoa(i+2)))
		}(i)
	}
	wg.Wait()
}
