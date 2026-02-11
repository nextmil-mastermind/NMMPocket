package lib

import (
	"log"
	"net/mail"
	"reflect"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/tools/mailer"
	"github.com/pocketbase/pocketbase/tools/types"
)

type Invoice struct {
	ID            string                `db:"id" mapstructure:"id"`
	Name          string                `db:"name" mapstructure:"name"`
	Email         string                `db:"email" mapstructure:"email"`
	InvoiceName   string                `db:"invoicename" mapstructure:"invoicename"`
	Description   string                `db:"description" mapstructure:"description"`
	Amount        float64               `db:"amount" mapstructure:"amount"`
	DueDate       types.DateTime        `db:"duedate" mapstructure:"duedate"`
	Session       string                `db:"session" mapstructure:"session"`
	Paid          bool                  `db:"paid" mapstructure:"paid"`
	Reminders     bool                  `db:"reminders" mapstructure:"reminders"`
	CC            *types.JSONArray[any] `db:"cc" mapstructure:"cc"`
	InvoiceType   string                `db:"type" mapstructure:"type"`
	SessionURL    string                `db:"sessionurl" mapstructure:"sessionurl"`
	DaysRemaining int                   `db:"days_remaining" mapstructure:"days_remaining"`
	Members       []string              `db:"members" mapstructure:"members"`
}

var InvoiceKeys = []string{
	"ID",
	"Name",
	"Email",
	"InvoiceName",
	"Description",
	"Amount",
}

type EmailTemplate struct {
	ID      string `db:"id"`
	Name    string `db:"name"`
	Subject string `db:"subject"`
	Body    string `db:"html"`
	Days    int    `db:"days"`
}

func CheckInvoice(app *pocketbase.PocketBase) {
	var res []Invoice
	err := app.DB().Select(
		"*, CAST(julianday(date(duedate)) - julianday(date('now')) as INTEGER) as days_remaining",
	).
		From("invoices").
		Where(dbx.NewExp("paid = {:paid}", dbx.Params{"paid": false})).
		AndWhere(dbx.NewExp("date(duedate) IN (date('now'), date('now', '+1 day'), date('now', '+5 day'), date('now', '+20 day'))", nil)).
		All(&res)
	if err != nil {
		log.Default().Println(err)
		return
	}
	if len(res) == 0 {
		return
	}
	templates := getEmailTemplates(app.DB())
	for _, invoice := range res {
		//check if the invoice has a member associated with it, if it does we need to grab the member
		hasMember := len(invoice.Members) > 0
		var memberTo mail.Address
		var memberCC []mail.Address
		if hasMember {
			var members []struct {
				First string `db:"first_name"`
				Last  string `db:"last_name"`
				Email string `db:"email"`
			}
			err := app.DB().Select("first_name, last_name, email").
				From("members").
				Where(dbx.NewExp("id IN ({:ids})", dbx.Params{"ids": invoice.Members})).
				All(&members)
			if err != nil {
				log.Default().Println(err)
			} else {
				for _, member := range members {
					if member.Email == "" {
						continue
					}
					name := strings.TrimSpace(member.First + " " + member.Last)
					if memberTo.Address == "" {
						memberTo = mail.Address{Address: member.Email, Name: name}
						continue
					}
					memberCC = append(memberCC, mail.Address{Address: member.Email, Name: name})
				}
				if len(members) > 0 {
					invoice.Name = strings.TrimSpace(members[0].First + " " + members[0].Last)
					invoice.Email = members[0].Email
				}
			}
		}
		if invoice.InvoiceType == "auto" && invoice.DaysRemaining == 0 {
			// Auto pay invoice
			_, err := createStripeCharge(invoice, app)
			if err != nil {
				log.Default().Println(err)
			}
		} else if invoice.Reminders {
			message := sendReminderEmail(invoice, templates[invoice.DaysRemaining], memberTo, memberCC)
			err := app.NewMailClient().Send(message)
			if err != nil {
				log.Default().Println(err)
			}
		} else {
			log.Default().Println("No email to send")
		}
	}
}

func sendReminderEmail(invoice Invoice, template EmailTemplate, to mail.Address, cc []mail.Address) *mailer.Message {
	messageText := paramsCleanUp(template.Body, invoice)
	if invoice.InvoiceType == "auto" {
		messageText = strings.ReplaceAll(messageText, "{{params.is_auto_pay}}", "<bold>Note:</bold> This invoice will be automatically billed to your card on the due date.<br>")
	} else {
		messageText = strings.ReplaceAll(messageText, "{{params.is_auto_pay}}", "")
	}
	//
	if to.Address == "" && invoice.Email != "" {
		to = mail.Address{Address: invoice.Email, Name: invoice.Name}
	}

	message := &mailer.Message{
		From: mail.Address{
			Address: "info@nextmilmastermind.com",
			Name:    "Next Mil Mastermind",
		},
		To:      []mail.Address{to},
		Cc:      cc,
		Subject: paramsCleanUp(template.Subject, invoice),
		HTML:    messageText,
	}
	return message
}

func getEmailTemplates(db dbx.Builder) map[int]EmailTemplate {
	var res []EmailTemplate
	db.Select("*").From("email_basic").Where(dbx.NewExp("is_invoice = true")).All(&res)
	var templates = make(map[int]EmailTemplate)
	for _, template := range res {
		templates[template.Days] = template
	}
	return templates
}

func paramsCleanUp(Text string, invoice Invoice) string {
	v := reflect.ValueOf(invoice)
	for _, key := range InvoiceKeys {
		field := v.FieldByName(key)
		if field.IsValid() && field.Kind() == reflect.String {
			Text = strings.ReplaceAll(Text, "{{params."+key+"}}", field.String())
		}
	}
	first_name := strings.Split(invoice.Name, " ")[0]
	Text = strings.ReplaceAll(Text, "{{params.first_name}}", first_name)
	Text = strings.ReplaceAll(Text, "{{params.DueDate}}", convertTimeToString(invoice.DueDate))
	return Text
}

func convertTimeToString(t types.DateTime) string {
	return t.Time().Format("Jan 2, 2006")
}
