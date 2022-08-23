package chantools

// Subscriber represents a subscriber which we multiplex messages to.
type Subscriber[T any] struct {
	id            uint64
	ch            chan T
	unSubscribeCh chan<- uint64
}

// Channel returns the read channel for this subscriber.
func (s *Subscriber[T]) Channel() <-chan T {
	return s.ch
}

// Close the subscriber by alerting the multiplexer we want to close
// the channel.  Readers should see channel become closed.
//
// TODO(borud): remove?
//
//	I am not sure having a Close() function is a good idea.
//	Technically the sender closes the channel here, but semantically
//	both subscriber and mux can close the subscription's channel.
func (s *Subscriber[T]) Close() {
	s.unSubscribeCh <- s.id
	<-s.ch
}
