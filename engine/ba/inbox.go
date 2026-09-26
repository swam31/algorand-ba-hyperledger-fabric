package ba

import "sync"

// Inbox buffers incoming votes indexed by (round, step).
//
// The paper's pseudocode says "we assume a background procedure takes incoming
// votes and stores them into that buffer, indexed by the messages' round and
// step". This is that buffer.
//
// Buffering by (round, step) rather than processing in arrival order is not an
// optimisation - it is required. Steps are NOT synchronised across nodes: a
// fast node can already be gossiping step 5 while a slow node is still counting
// step 3. Without a buffer those early votes would be discarded and the slow
// node could never catch up.
type Inbox struct {
	mu      sync.Mutex
	buckets map[[2]uint64]*bucket
}

type bucket struct {
	votes  []*Vote
	notify chan struct{} // closed and replaced whenever a vote is appended
}

func NewInbox() *Inbox {
	return &Inbox{buckets: make(map[[2]uint64]*bucket)}
}

func (i *Inbox) get(round, step uint64) *bucket {
	k := [2]uint64{round, step}
	b, ok := i.buckets[k]
	if !ok {
		b = &bucket{notify: make(chan struct{})}
		i.buckets[k] = b
	}
	return b
}

// Add files a vote under its own round and step.
func (i *Inbox) Add(v *Vote) {
	i.mu.Lock()
	defer i.mu.Unlock()
	b := i.get(v.Round, v.Step)
	b.votes = append(b.votes, v)
	close(b.notify)                 // wake everyone waiting on this bucket
	b.notify = make(chan struct{})  // and arm the next wait
}

// Since returns votes for (round, step) beyond index `from`, together with a
// channel that closes when more arrive. Callers poll with:
//
//	for {
//	    vs, more := inbox.Since(r, s, seen)
//	    seen += len(vs)
//	    ... process vs ...
//	    select { case <-more: case <-timeout: return }
//	}
func (i *Inbox) Since(round, step uint64, from int) ([]*Vote, <-chan struct{}) {
	i.mu.Lock()
	defer i.mu.Unlock()
	b := i.get(round, step)
	if from >= len(b.votes) {
		return nil, b.notify
	}
	out := make([]*Vote, len(b.votes)-from)
	copy(out, b.votes[from:])
	return out, b.notify
}

// All returns every vote currently held for (round, step). Used by CommonCoin,
// which needs a snapshot rather than a stream.
func (i *Inbox) All(round, step uint64) []*Vote {
	vs, _ := i.Since(round, step, 0)
	return vs
}

// Prune drops every bucket for rounds strictly below keepFrom, so a long-running
// node does not accumulate the entire vote history in memory.
func (i *Inbox) Prune(keepFrom uint64) {
	i.mu.Lock()
	defer i.mu.Unlock()
	for k := range i.buckets {
		if k[0] < keepFrom {
			delete(i.buckets, k)
		}
	}
}
