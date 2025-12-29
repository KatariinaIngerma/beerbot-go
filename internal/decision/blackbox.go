package decision

// BlackBoxOrder: a simple, deterministic blackbox heuristic.
//
// Idea:
// - Forecast demand using a moving average of incoming_orders (last N weeks).
// - Add current backlog (unmet demand).
// - Maintain a small safety stock buffer.
// - Order enough to cover forecast + backlog + safety - current inventory.
// - Clamp at 0 (non-negative integer requirement). :contentReference[oaicite:5]{index=5}
func BlackBoxOrder(history []RoleState, safetyStock int, window int) int {
	if len(history) == 0 {
		// If no history, return a safe baseline.
		return 10
	}
	last := history[len(history)-1]

	forecast := movingAverageIncomingOrders(history, window)

	order := forecast + last.Backlog + safetyStock - last.Inventory
	if order < 0 {
		order = 0
	}
	return order
}

func movingAverageIncomingOrders(history []RoleState, window int) int {
	if window < 1 {
		window = 1
	}
	n := len(history)
	start := n - window
	if start < 0 {
		start = 0
	}
	sum := 0
	count := 0
	for i := start; i < n; i++ {
		sum += history[i].IncomingOrders
		count++
	}
	if count == 0 {
		return 0
	}
	// Integer average (deterministic)
	return sum / count
}
