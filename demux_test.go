package chantools

import (
	"errors"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMultiplexer(t *testing.T) {
	mchan := make(chan int)
	go func() {
		for i := 0; i < 10; i++ {
			mchan <- i
		}
	}()

	m, err := NewDemux(mchan)
	require.NoError(t, err)

	// Close the multiplexer after 20 ms
	go func() {
		time.Sleep(20 * time.Millisecond)
		m.Close()
	}()

	// create 5 subscribers
	for i := 0; i < 5; i++ {
		sub, err := m.Subscribe()
		if err != nil {
			assert.Error(t, ErrDemuxClosed)
			return
		}

		// at this point the only acceptable errors are ErrMultiplexerClosed or nil
		if errors.Is(err, ErrDemuxClosed) {
			return
		}
		require.NoError(t, err)

		go func(s *Subscriber[int]) {
			// range slowly over channel so we have some chance of being shut down mid way through.
			for m := range s.Channel() {
				log.Printf("%+v", m)
				time.Sleep(5 * time.Millisecond)
			}
		}(sub)
	}

}

func TestUpstreamChannelClose(t *testing.T) {
	mchan := make(chan int)
	go func() {
		for i := 0; i < 5; i++ {
			mchan <- i
		}
		close(mchan)
	}()

	mplex, err := NewDemux(mchan)
	require.NoError(t, err)
	assert.Equal(t, "*chantools.Demux[int]", reflect.TypeOf(mplex).String())

	select {
	case <-mplex.closedCh:
	case <-time.After(100 * time.Millisecond):
		assert.FailNow(t, "timed out")
	}
}
