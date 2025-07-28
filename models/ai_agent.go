package models

type AIAgentData struct {
	ID          string `json:"id" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required"`
	URL         string `json:"url" validate:"required,url"`
}

type ClientAgentData struct {
	ID          string `json:"id" validate:"required"`
	Name        string `json:"name" validate:"required"`
	Description string `json:"description" validate:"required"`
	URL         string `json:"url" validate:"required,url"`
}
