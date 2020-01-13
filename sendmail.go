package sendmail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/mail"
	"github.com/h2non/filetype"
	"net/smtp"
	"strings"
	"text/template"
	"path/filepath"
	"encoding/base64"
)

//var Message mail.Message

type Config struct {
	User     string
	Password string
	Host     string
	Port     int
	TLS      bool
	Datagram string
}

func headerToString(h *mail.Header) string {
	head := ""
	for k, v := range *h {
		head = strings.Join([]string{head, k, ": ", strings.Join(v, " "), "\r\n"}, "")
	}
	log.Println("Header data: \n", head)
	return head
}

func dial(setting *Config) (net.Conn, error) {
	host := fmt.Sprintf("%s:%d", setting.Host, setting.Port)
	if setting.TLS {
		tlsConfig := tls.Config{
			ServerName:         setting.Host,
			InsecureSkipVerify: true,
		}
		return tls.Dial(setting.Datagram, host, &tlsConfig)
	}
	return net.Dial(setting.Datagram, host)
}

func newClient(conn net.Conn, setting *Config) (*smtp.Client, error) {
	return smtp.NewClient(conn, setting.Host)
}

func authClient(client *smtp.Client, setting *Config) error {
	auth := smtp.PlainAuth("", setting.User, setting.Password, setting.Host)
	return client.Auth(auth)
}

func getDataWriter(client *smtp.Client, mailHeader *mail.Header) (io.WriteCloser, error) {
	log.Println("Set 'FROM:' ")
	formEMail, err := mail.ParseAddress(mailHeader.Get("From"))
	if err != nil {
		log.Panic(err)
	}
	if err := client.Mail(formEMail.Address); err != nil {
		log.Panic(err)
	}
	log.Println(formEMail.Address)
	log.Println("Set 'TO(s)':")
	toEMails, err := mail.ParseAddressList(mailHeader.Get("To"))
	if err != nil {
		log.Panic(err)
	}
	for _, to := range toEMails {
		if err := client.Rcpt(to.Address); err != nil {
			log.Panic(err)
		}
		log.Println(to.Address)
	}
	return client.Data()
}

func Send(setting *Config, msg *mail.Message) {
	log.Printf("Establish %s:%s:%d SMTP connection", setting.Datagram, setting.Host, setting.Port)
	conn, connErr := dial(setting)
	if connErr != nil {
		log.Panic(connErr)
	}
	defer conn.Close()
	log.Println("Create new email client")
	client, clientErr := newClient(conn, setting)
	if clientErr != nil {
		log.Panic(clientErr)
	}
	defer client.Close()
	log.Println("Setup authenticate credential")
	if errAuth := authClient(client, setting); errAuth != nil {
		log.Panic(errAuth)
	}
	log.Println("Start write mail content")
	writer, writerErr := getDataWriter(client, &msg.Header)
	if writerErr != nil {
		log.Panic(writerErr)
	}
	//basic email headers to msg string
	buffHeader := headerToString(&msg.Header)
	bufBody, errBody := ioutil.ReadAll(msg.Body)
	if errBody != nil {
		log.Fatal(errBody)
	}
	log.Println("Write content into client writter I/O")
	buffMsg := bytes.Join([][]byte{[]byte(buffHeader), bufBody}, []byte("\r\n"))
	if _, err := writer.Write(buffMsg); err != nil {
		log.Panic(err)
	}

	if closeErr := writer.Close(); closeErr != nil {
		log.Panic(closeErr)
	}

	client.Quit()

	log.Print("done.")
}
func SendMail(setting *Config, msg *string){
	mailMsg, errMailMsg := mail.ReadMessage(strings.NewReader(*msg))
	if errMailMsg != nil {
		log.Panic(errMailMsg)
	}
	Send(setting, mailMsg)
}
func ReadTemplateFile(path string, data interface{}) (string, error) {
	var buff bytes.Buffer
	//Read template
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		log.Panic(err)
		return "<nil>", err
	}
	err = tmpl.Execute(&buff, &data)
	if err != nil {
		log.Panic(err)
		return "<nil>", err
	}
	return buff.String(), nil
}
func ContentHTML(html string) string {
	mailHeader := make(mail.Header);
	mailHeader["Content-Type"] = []string{"text/html;", "charset=\""+"utf-8"+"\""}
	mailHeader["Content-Transfer-Encoding"] = []string{"7bit"}
	mailHtml := strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(html, "\n", ""), "\r", ""), "\t", "")
	return fmt.Sprintf("%s\r\n%s\r\n", headerToString(&mailHeader), mailHtml)
}
func AttachedFile(fpath string, boundary string) string {
	//read file
	attachRawFile, fileErr := ioutil.ReadFile(fpath)
	if fileErr != nil {
		log.Panic(fileErr)
		return "<nil>"
	}

	kindFile, _  := filetype.Match(attachRawFile)
	if kindFile == filetype.Unknown {
		log.Println("Unknown file type")
	}

	log.Println("filetype:", kindFile.MIME.Value)

	filename := filepath.Base(fpath)
	mailFile := make(mail.Header);
	mailFile["Content-Type"] = []string{kindFile.MIME.Value}
	mailFile["Content-Transfer-Encoding"] = []string{"base64"}
	mailFile["Content-Disposition"] = []string{"attachment;", "filename=\""+filename+"\""}
	fileHeader := fmt.Sprintf("--%s\r\n", boundary) + headerToString(&mailFile)
	return fileHeader + "\r\n" + base64.StdEncoding.EncodeToString(attachRawFile)
}
