package events

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestBus_FanoutSameOrg(t *testing.T) {
	b := NewBus()
	org := uuid.New()
	s1 := b.Subscribe(org, 4)
	s2 := b.Subscribe(org, 4)
	defer b.Unsubscribe(s1)
	defer b.Unsubscribe(s2)

	b.Publish(Event{OrgID: org, Topic: "ticket.claim"})

	for i, ch := range []chan Event{s1.Ch, s2.Ch} {
		select {
		case ev := <-ch:
			if ev.Topic != "ticket.claim" {
				t.Fatalf("sub %d: topic=%s", i, ev.Topic)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("sub %d: timeout esperando evento", i)
		}
	}
}

func TestBus_DifferentOrgsIsolated(t *testing.T) {
	b := NewBus()
	orgA := uuid.New()
	orgB := uuid.New()
	sA := b.Subscribe(orgA, 4)
	sB := b.Subscribe(orgB, 4)
	defer b.Unsubscribe(sA)
	defer b.Unsubscribe(sB)

	b.Publish(Event{OrgID: orgA, Topic: "ticket.claim"})

	select {
	case ev := <-sA.Ch:
		if ev.OrgID != orgA {
			t.Fatal("sA recibió evento de otra org")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("sA timeout")
	}

	select {
	case ev := <-sB.Ch:
		t.Fatalf("sB NO debería recibir nada (es de orgB), got %v", ev)
	case <-time.After(50 * time.Millisecond):

	}
}

func TestBus_LossyWhenFull(t *testing.T) {
	b := NewBus()
	org := uuid.New()
	s := b.Subscribe(org, 2)
	defer b.Unsubscribe(s)

	for i := 0; i < 5; i++ {
		b.Publish(Event{OrgID: org, Topic: "ticket.update"})
	}
	got := 0
	for {
		select {
		case <-s.Ch:
			got++
		case <-time.After(50 * time.Millisecond):
			if got != 2 {
				t.Fatalf("expected 2 events (buffer size), got %d", got)
			}
			return
		}
	}
}

func TestBus_UnsubscribeClosesChannel(t *testing.T) {
	b := NewBus()
	org := uuid.New()
	s := b.Subscribe(org, 4)
	b.Unsubscribe(s)
	if _, ok := <-s.Ch; ok {
		t.Fatal("expected closed channel after unsubscribe")
	}
}

func TestBus_ConcurrentSubsAndPublish(t *testing.T) {
	b := NewBus()
	org := uuid.New()
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := b.Subscribe(org, 16)
			defer b.Unsubscribe(s)
			<-s.Ch
		}()
	}
	time.Sleep(10 * time.Millisecond) // let subs register
	b.Publish(Event{OrgID: org, Topic: "ticket.create"})
	wg.Wait()
}
