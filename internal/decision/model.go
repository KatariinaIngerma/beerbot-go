package decision

type RoleState struct {
	Inventory         int `json:"inventory"`
	Backlog           int `json:"backlog"`
	IncomingOrders    int `json:"incoming_orders"`
	ArrivingShipments int `json:"arriving_shipments"`
}

type WeekState struct {
	Week   int                  `json:"week"`
	Roles  map[string]RoleState `json:"roles"`
	Orders map[string]int       `json:"orders"`
}

// ExtractRoleHistory returns a slice of RoleState for a given role across all weeks.
// This supports the “stateless but adaptive” approach since all history arrives in the request. :contentReference[oaicite:4]{index=4}
func ExtractRoleHistory(weeks []WeekState, role string) []RoleState {
	out := make([]RoleState, 0, len(weeks))
	for _, w := range weeks {
		rs, ok := w.Roles[role]
		if !ok {
			out = append(out, RoleState{})
			continue
		}
		out = append(out, rs)
	}
	return out
}
