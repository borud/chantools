package chantools

import (
	"errors"
	"log"
	"sync/atomic"
	"time"
)

type message interface {
	any
}

// Demux creates a pub/sub like demux for a read channel.
type Demux[T message] struct {
	ch              <-chan T
	subscribeCh     chan Subscriber[T]
	unSubscribeCh   chan uint64
	deliveryTimeout time.Duration
	lastSubcriberID uint64
	subscribers     []Subscriber[T]
	isClosed        atomic.Value
	shutdownCh      chan struct{}
	closedCh        chan struct{}
}

const (
	defaultDeliveryTimeout               = 5 * time.Second
	maxDuration            time.Duration = 1<<63 - 1
)

// Errors
var (
	ErrDemuxClosed = errors.New("demux closed")
	ErrTimeout     = errors.New("operation timed out")
)

// Create a demux for a channel.
func Create[T message](ch chan T) (*Demux[T], error) {
	m := &Demux[T]{
		ch:              ch,
		subscribeCh:     make(chan Subscriber[T]),
		unSubscribeCh:   make(chan uint64),
		deliveryTimeout: defaultDeliveryTimeout,
		lastSubcriberID: 0,
		subscribers:     []Subscriber[T]{},
		isClosed:        atomic.Value{},
		shutdownCh:      make(chan struct{}),
		closedCh:        make(chan struct{}),
	}

	go m.readLoop()
	return m, nil
}

func (m *Demux[T]) readLoop() {
	// Do all the cleanup here so we only have to return from the selection loop.
	defer func() {
		log.Printf("readLoop exits")

		for _, sub := range m.subscribers {
			log.Printf(" - closing sub %d", sub.id)
			close(sub.ch)
		}
		close(m.closedCh)
	}()

	for {
		select {
		// when message arrives from the channel we demux
		case msg, ok := <-m.ch:
			if !ok {
				log.Printf("upstream channel closed")
				return
			}

			for _, sub := range m.subscribers {
				// make this nonblocking
				sub.ch <- msg
			}

		// when new subscribe arrives
		case sub := <-m.subscribeCh:
			m.subscribers = append(m.subscribers, sub)
			log.Printf("readLoop subscribe: %+v", sub)

		// remove subscriber by id
		case unSubID := <-m.unSubscribeCh:
			for i, sub := range m.subscribers {
				if sub.id == unSubID {
					close(sub.ch)
					m.subscribers = append(m.subscribers[:i], m.subscribers[i+1:]...)
					log.Printf("removed subscriber %d", sub.id)
				}
			}

		// when shutdown is called the shutdownCh is closed
		case <-m.shutdownCh:
			return
		}
	}
}

// Subscribe creates a subscriber
func (m *Demux[T]) Subscribe(timeout ...time.Duration) (*Subscriber[T], error) {
	if m.isClosed.Load() != nil {
		return nil, ErrDemuxClosed
	}

	to := maxDuration
	if len(timeout) > 0 {
		to = timeout[0]
	}

	sub := Subscriber[T]{
		id:            atomic.AddUint64(&m.lastSubcriberID, 1),
		ch:            make(chan T),
		unSubscribeCh: m.unSubscribeCh,
	}

	select {
	case <-m.closedCh:
		return nil, ErrDemuxClosed

	case m.subscribeCh <- sub:
		return &sub, nil

	case <-time.After(to):
		return nil, ErrTimeout
	}
}

// Close the demux
func (m *Demux[T]) Close() error {
	m.isClosed.Store(struct{}{})
	close(m.shutdownCh)
	<-m.closedCh
	close(m.subscribeCh)
	close(m.unSubscribeCh)
	log.Printf("closed")
	return nil
}
