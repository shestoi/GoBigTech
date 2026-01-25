package templates

import (
	"bytes"
	"fmt"
	"text/template"

	"go.uber.org/zap"
)

// Renderer рендерит шаблоны для уведомлений
type Renderer struct {
	logger              *zap.Logger
	paymentTemplate     *template.Template
	assemblyTemplate    *template.Template
}

// NewRenderer создаёт новый renderer и загружает шаблоны
func NewRenderer(logger *zap.Logger, templatesDir string) (*Renderer, error) {
	paymentTemplate, err := template.ParseFiles(templatesDir + "/payment_completed.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse payment template: %w", err)
	}

	assemblyTemplate, err := template.ParseFiles(templatesDir + "/assembly_completed.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse assembly template: %w", err)
	}

	return &Renderer{
		logger:           logger,
		paymentTemplate:  paymentTemplate,
		assemblyTemplate: assemblyTemplate,
	}, nil
}

// RenderPaymentCompleted рендерит шаблон для события оплаты заказа
func (r *Renderer) RenderPaymentCompleted(data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := r.paymentTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render payment template: %w", err)
	}
	return buf.String(), nil
}

// RenderAssemblyCompleted рендерит шаблон для события завершения сборки заказа
func (r *Renderer) RenderAssemblyCompleted(data interface{}) (string, error) {
	var buf bytes.Buffer
	if err := r.assemblyTemplate.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to render assembly template: %w", err)
	}
	return buf.String(), nil
}

