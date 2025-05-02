package dto

// ConfigDto дто для эндпоинтов
type ConfigDto struct {
	ClientId       string `json:"client_id"`
	Capacity       int    `json:"capacity"`
	RefillInterval string `json:"refill_interval"`
}
