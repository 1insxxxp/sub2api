package handler

import "testing"

func TestProvideHandlersIncludesModelStatusHandler(t *testing.T) {

	userCustomGroupHandler := &UserCustomGroupHandler{}
	checkinHandler := &CheckinHandler{}
	imageStudioHandler := &ImageStudioHandler{}
	modelStatusHandler := &ModelStatusHandler{}
	lotteryHandler := &LotteryHandler{}
	handlers := ProvideHandlers(
		nil, nil, nil, userCustomGroupHandler, nil, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
		checkinHandler, imageStudioHandler, nil, modelStatusHandler, nil, nil,
		lotteryHandler, nil, nil, nil,
	)

	if handlers.ModelStatus != modelStatusHandler {
		t.Fatalf("expected model status handler to be wired into Handlers")
	}
	if handlers.UserCustomGroup != userCustomGroupHandler {
		t.Fatalf("expected user custom group handler to be wired into Handlers")
	}
	if handlers.Checkin != checkinHandler {
		t.Fatalf("expected check-in handler to be wired into Handlers")
	}
	if handlers.ImageStudio != imageStudioHandler {
		t.Fatalf("expected image studio handler to be wired into Handlers")
	}
	if handlers.Lottery != lotteryHandler {
		t.Fatalf("expected lottery handler to be wired into Handlers")
	}
}
