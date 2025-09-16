package email

import (
	"fmt"
	"nmmpocket/lib"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

func RegisterMailer(app *pocketbase.PocketBase) {
	app.OnMailerSend().BindFunc(func(e *core.MailerEvent) error {

		// Convert MailerEvent to lib.EmailSender format
		var recipients []lib.Recipient

		// Extract recipients from the mailer event
		for _, to := range e.Message.To {
			recipient := lib.Recipient{
				Email: to.Address,
				Name:  to.Name,
			}
			// Use email as name if no name is provided
			if recipient.Name == "" {
				recipient.Name = to.Address
			}
			recipients = append(recipients, recipient)
		}
		from := lib.Contact{
			Email: e.Message.From.Address,
			Name:  e.Message.From.Name,
		}
		fmt.Printf("Sending email from %s to %d recipients\n", from.Email, len(recipients))
		// Send email using lib.EmailSender instead of pocketbase mailer
		err := lib.EmailSenderFrom(from, from, recipients, e.Message.Subject, e.Message.HTML, nil)
		if err != nil {
			return err
		}

		// Return nil to skip the default mailer
		return nil
	})

}
