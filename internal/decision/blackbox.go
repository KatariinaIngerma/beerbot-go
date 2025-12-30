package decision

// BlackBoxOrder: a simple, deterministic blackbox heuristic.
//
// Idea:
// - Forecast demand using a moving average of incoming_orders (last N weeks).
// - Add current backlog (unmet demand).
// - Maintain a small safety stock buffer.
// - Order enough to cover forecast + backlog + safety - current inventory.
// - Clamp at 0 (non-negative integer requirement). :contentReference[oaicite:5]{index=5}
func BlackBoxOrderWithPipeline(roleHistory []RoleState, roleOrders []int, safetyStock int, window int) int {
	if len(roleHistory) == 0 {
		return 10
	}

	last := roleHistory[len(roleHistory)-1]
	forecast := movingAverageIncomingOrders(roleHistory, window)

	// pipeline = sum(orders) - sum(arriving_shipments)
	pipeline := 0
	for i := 0; i < len(roleHistory); i++ {
		if i < len(roleOrders) {
			pipeline += roleOrders[i]
		}
		pipeline -= roleHistory[i].ArrivingShipments
		if pipeline < 0 {
			pipeline = 0
		}
	}

	// assume lead time L=2 (common Beer Game)
	L := 2
	targetPosition := forecast * (L + 1)
	inventoryPosition := last.Inventory - last.Backlog + pipeline

	order := targetPosition - inventoryPosition
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
