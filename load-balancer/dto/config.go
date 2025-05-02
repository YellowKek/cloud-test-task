package dto

type ConfigDto struct {
	ClientId       string `json:"client_id"`
	Capacity       int    `json:"capacity"`
	RefillInterval string `json:"refill_interval"`
}
