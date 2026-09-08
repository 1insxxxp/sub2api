package handler

import "testing"

func TestProvideHandlersIncludesModelStatusHandler(t *testing.T) {

	userCustomGroupHandler := &UserCustomGroupHandler{}
	modelStatusHandler := &ModelStatusHandler{}
	handlers := ProvideHandlers(
		nil, nil, nil, userCustomGroupHandler, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
		modelStatusHandler, nil, nil, nil, nil, nil,
	)

	if handlers.ModelStatus != modelStatusHandler {
		t.Fatalf("expected model status handler to be wired into Handlers")
	}
	if handlers.UserCustomGroup != userCustomGroupHandler {
		t.Fatalf("expected user custom group handler to be wired into Handlers")
	}
}
