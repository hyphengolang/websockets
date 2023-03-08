package structures

import "sync"

type SyncMap[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]V
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	m := &SyncMap[K, V]{
		m: make(map[K]V),
	}
	return m
}

func (m *SyncMap[K, V]) Load(key K) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.m[key]
	return v, ok
}

func (m *SyncMap[K, V]) Store(key K, value V) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[key] = value
}

func (m *SyncMap[K, V]) Delete(key K) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, key)
}

func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.m {
		if !f(k, v) {
			break
		}
	}
}

func (m *SyncMap[K, V]) Len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.m)
}

func (m *SyncMap[K, V]) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m = make(map[K]V)
}

func (m *SyncMap[K, V]) Keys() []K {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]K, 0, len(m.m))
	for k := range m.m {
		keys = append(keys, k)
	}
	return keys
}

func (m *SyncMap[K, V]) Values() []V {
	m.mu.Lock()
	defer m.mu.Unlock()
	values := make([]V, 0, len(m.m))
	for _, v := range m.m {
		values = append(values, v)
	}
	return values
}

func (m *SyncMap[K, V]) Items() []struct {
	K K
	V V
} {
	m.mu.Lock()
	defer m.mu.Unlock()
	items := make([]struct {
		K K
		V V
	}, 0, len(m.m))
	for k, v := range m.m {
		items = append(items, struct {
			K K
			V V
		}{k, v})
	}
	return items
}

func (m *SyncMap[K, V]) Has(key K) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.m[key]
	return ok
}

func (m *SyncMap[K, V]) IsEmpty() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.m) == 0
}

func (m *SyncMap[K, V]) Clone() *SyncMap[K, V] {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := NewSyncMap[K, V]()
	for k, v := range m.m {
		n.Store(k, v)
	}
	return n
}

func (m *SyncMap[K, V]) Merge(n *SyncMap[K, V]) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n.mu.Lock()
	defer n.mu.Unlock()
	for k, v := range n.m {
		m.Store(k, v)
	}
}

// func (m *SyncMap[K, V]) Equal(n *SyncMap[K, V]) bool {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	n.mu.Lock()
// 	defer n.mu.Unlock()
// 	if len(m.m) != len(n.m) {
// 		return false
// 	}
// 	for k, v := range m.m {
// 		if nv, ok := n.m[k]; !ok || nv != v {
// 			return false
// 		}
// 	}
// 	return true
// }

// func (m *SyncMap[K, V]) String() string {
// 	m.mu.Lock()
// 	defer m.mu.Unlock()
// 	str := "SyncMap{"
// 	for k, v := range m.m {
// 		str += k.String() + ":" + v.String() + ","
// 	}
// 	str += "}"
// 	return str
// }
