package tui

// The queue panel lists the queue as Spotify does: the track playing,
// then what comes next in the order it will play, shuffle included,
// then what has played, newest first. queueCursor and queueOffset are
// places in that list; these turn them into places in the queue and
// back.

// queueOrder is the queue in the panel's order, as indices into
// queue.Tracks, and how many of them are the playing track and what
// comes next (the rest have played).
func (m Model) queueOrder() (order []int, live int) {
	cur, next, played := m.queue.Order()
	if cur >= 0 {
		order = append(order, cur)
	}
	order = append(order, next...)
	live = len(order)
	for i := len(played) - 1; i >= 0; i-- {
		order = append(order, played[i])
	}
	return order, live
}

// queueIndexAt is the queue index of the track at place pos in the
// panel, or -1.
func (m Model) queueIndexAt(pos int) int {
	order, _ := m.queueOrder()
	if pos < 0 || pos >= len(order) {
		return -1
	}
	return order[pos]
}

// queuePlaceOf is where the queue's track i sits in the panel, or 0.
func (m Model) queuePlaceOf(i int) int {
	order, _ := m.queueOrder()
	for pos, at := range order {
		if at == i {
			return pos
		}
	}
	return 0
}

// selectedQueueIndex is the queue index of the track under the cursor,
// or -1.
func (m Model) selectedQueueIndex() int {
	return m.queueIndexAt(m.queueCursor)
}
