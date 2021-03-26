package bytepool

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBytePool(t *testing.T) {
	p := NewBytePool(2, 3, 0)
	require.Equal(t, 2, len(p.ch))
	require.Equal(t, 3, p.BufferSize())
	b := p.Get()
	require.Equal(t, []byte{0, 0, 0}, b)
	require.Equal(t, 1, len(p.ch))
	p.Get()
	// ch empty, get timeout
	ctx, cancel := context.WithTimeout(context.TODO(), 100*time.Millisecond)
	defer cancel()
	_, err := p.GetWithCtx(ctx)
	require.Equal(t, context.DeadlineExceeded, err)
	// change data and get the same data
	b[0] = 1
	p.Put(b)
	require.Equal(t, 1, len(p.ch))
	// put same buffer no effect
	p.Put(b)
	require.Equal(t, 1, len(p.ch))
	b2 := p.Get()
	require.Equal(t, []byte{1, 0, 0}, b2)
	b = nil
	// forgot to reput, gc
	runtime.GC()
	require.Equal(t, 2, len(p.ch))
	p.Put(nil)
	require.Equal(t, 2, len(p.ch))
}

func TestGC(t *testing.T) {
	p := NewBytePool(1, 3, 100*time.Millisecond)
	b := p.Get()
	b[0] = 1
	p.Put(b)
	b = p.Get()
	require.Equal(t, []byte{1, 0, 0}, b)
	p.Put(b)
	b = nil
	time.Sleep(150 * time.Millisecond)
	b = p.Get()
	require.Equal(t, []byte{0, 0, 0}, b)
}
