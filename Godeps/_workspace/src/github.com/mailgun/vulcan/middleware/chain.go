package middleware

import (
	"fmt"
	. "github.com/mailgun/vulcan/request"
	"sort"
	"sync"
)

// Middleware chain implements middleware interface and acts as a container
// for multiple middlewares chained together in deterministic order.
type MiddlewareChain struct {
	chain *chain
}

func NewMiddlewareChain() *MiddlewareChain {
	return &MiddlewareChain{
		chain: newChain(),
	}
}

func (c *MiddlewareChain) Add(id string, priority int, m Middleware) error {
	return c.chain.append(id, priority, m)
}

func (c *MiddlewareChain) Upsert(id string, priority int, m Middleware) {
	c.chain.upsert(id, priority, m)
}

func (c *MiddlewareChain) Remove(id string) error {
	return c.chain.remove(id)
}

func (c *MiddlewareChain) Update(id string, priority int, m Middleware) error {
	return c.chain.update(id, priority, m)
}

func (c *MiddlewareChain) Get(id string) Middleware {
	m := c.chain.get(id)
	if m != nil {
		return m.(Middleware)
	}
	return nil
}

func (c *MiddlewareChain) GetIter() *MiddlewareIter {
	return &MiddlewareIter{
		iter: c.chain.getIter(),
	}
}

type MiddlewareIter struct {
	iter *iter
}

func (m *MiddlewareIter) Next() Middleware {
	val := m.iter.next()
	if val == nil {
		return nil
	}
	return val.(Middleware)
}

func (m *MiddlewareIter) Prev() Middleware {
	val := m.iter.prev()
	if val == nil {
		return nil
	}
	return val.(Middleware)
}

type ObserverChain struct {
	chain *chain
}

func NewObserverChain() *ObserverChain {
	return &ObserverChain{
		chain: newChain(),
	}
}

func (c *ObserverChain) Add(id string, o Observer) error {
	return c.chain.append(id, 0, o)
}

func (c *ObserverChain) Upsert(id string, o Observer) {
	c.chain.upsert(id, 0, o)
}

func (c *ObserverChain) Remove(id string) error {
	return c.chain.remove(id)
}

func (c *ObserverChain) Update(id string, o Observer) error {
	return c.chain.update(id, 0, o)
}

func (c *ObserverChain) Get(id string) Observer {
	o := c.chain.get(id)
	if o != nil {
		return o.(Observer)
	}
	return nil
}

func (c *ObserverChain) ObserveRequest(r Request) {
	it := c.chain.getIter()
	for v := it.next(); v != nil; v = it.next() {
		v.(Observer).ObserveRequest(r)
	}
}

func (c *ObserverChain) ObserveResponse(r Request, a Attempt) {
	it := c.chain.getReverseIter()
	for v := it.next(); v != nil; v = it.next() {
		v.(Observer).ObserveResponse(r, a)
	}
}

// Map with guaranteed iteration order, in place updates that do not change the order
// and iterator that does not hold locks
type chain struct {
	mutex     *sync.RWMutex
	callbacks []*callback
	indexes   map[string]int // Indexes for in place updates
	iter      *iter          //current version of iterator
}

type callback struct {
	id       string
	priority int
	cb       interface{}
}

type callbacks []*callback

func (c callbacks) Len() int {
	return len(c)
}

func (c callbacks) Less(i, j int) bool {
	return c[i].priority < c[j].priority
}

func (c callbacks) Swap(i, j int) {
	c[i], c[j] = c[j], c[i]
}

func newChain() *chain {
	return &chain{
		mutex:     &sync.RWMutex{},
		callbacks: callbacks{},
	}
}

func (c *chain) append(id string, priority int, cb interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if p, _ := c.find(id); p != nil {
		return fmt.Errorf("Callback with id: %s already exists", id)
	}
	c.callbacks = append(c.callbacks, &callback{id, priority, cb})
	sort.Stable((callbacks)(c.callbacks))
	return nil
}

func (c *chain) find(id string) (*callback, int) {
	for i, c := range c.callbacks {
		if c.id == id {
			return c, i
		}
	}
	return nil, -1
}

func (c *chain) update(id string, priority int, cb interface{}) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	p, _ := c.find(id)
	if p == nil {
		return fmt.Errorf("Callback with id: %s not found", id)
	}
	p.cb = cb
	p.priority = priority
	sort.Stable((callbacks)(c.callbacks))
	return nil
}

func (c *chain) upsert(id string, priority int, cb interface{}) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	p, _ := c.find(id)
	if p == nil {
		c.callbacks = append(c.callbacks, &callback{id, priority, cb})
	} else {
		p.cb = cb
		p.priority = priority
	}
	sort.Stable((callbacks)(c.callbacks))
}

func (c *chain) get(id string) interface{} {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	p, _ := c.find(id)
	if p == nil {
		return nil
	} else {
		return p.cb
	}
}

func (c *chain) remove(id string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	p, i := c.find(id)
	if p == nil {
		return fmt.Errorf("Callback with id: %s not found", id)
	}
	c.callbacks = append(c.callbacks[:i], c.callbacks[i+1:]...)
	sort.Stable((callbacks)(c.callbacks))
	return nil
}

// Note that we hold read lock to get access to the current iterator
func (c *chain) getIter() *iter {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return newIter(c.callbacks)
}

func (c *chain) getReverseIter() *reverseIter {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return &reverseIter{callbacks: c.callbacks}
}

func newIter(callbacks []*callback) *iter {
	return &iter{
		index:     -1,
		callbacks: callbacks,
	}
}

type iter struct {
	index     int
	callbacks []*callback
}

func (it *iter) next() interface{} {
	if it.index >= len(it.callbacks) {
		return nil
	}
	it.index += 1
	if it.index >= len(it.callbacks) {
		return nil
	}
	return it.callbacks[it.index].cb
}

func (it *iter) prev() interface{} {
	if it.index < 0 {
		return nil
	}
	it.index -= 1
	if it.index < 0 {
		return nil
	}
	return it.callbacks[it.index].cb
}

type reverseIter struct {
	index     int
	callbacks []*callback
}

func (it *reverseIter) next() interface{} {
	if it.index >= len(it.callbacks) {
		return nil
	}
	val := it.callbacks[len(it.callbacks)-it.index-1].cb
	it.index += 1
	return val
}
