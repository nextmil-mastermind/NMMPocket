package event

import "fmt"

func EventResponseProcess(intent map[string]any) error {
	sessionID, ok := intent["id"].(string)
	if !ok {
		return fmt.Errorf("invalid session ID")
	}
	checkout, err := selectCheckout(sessionID)
	if err != nil {
		return fmt.Errorf("failed to select checkout: %v", err)
	}
	if checkout.Processed {
		return fmt.Errorf("checkout already processed")
	}
	AddOrder(checkout, float64(intent["amount"].(int64))/100)
	return nil

}
