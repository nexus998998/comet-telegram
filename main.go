package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	tg "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB
var bot *tg.BotAPI

type session struct {
	Grade           int
	Exam            int
	Subject         int
	LookingForTitle bool
	Title           string
	Type            string
}

type pendingUpload struct {
	ID      string
	UserID  int64
	Grade   int
	Exam    int
	Subject int
	Title   string
	FileID  string
	ChatID  int64
}

type examConfig struct {
	Exam    int
	Grade   int
	Subject int
}

type File struct {
	ID     string
	Title  string
	FileID string
}

type admin struct {
	TgID string
}

var sessions = map[int64]*session{}
var pendingUploads = map[string]*pendingUpload{}

var adminCounter int

var adminChatIDS = []int64{1743591825}

var startMarkup = tg.NewInlineKeyboardMarkup(
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("عرض امتحان", "showExam:"),
		tg.NewInlineKeyboardButtonData("رفع امتحان", "uploadExam:"),
	),
)

var subjectMarkup = tg.NewInlineKeyboardMarkup(
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("عربي", "subject:1"),
		tg.NewInlineKeyboardButtonData("رياضيات", "subject:2"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("كيمياء", "subject:3"),
		tg.NewInlineKeyboardButtonData("فيزياء", "subject:4"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("احياء", "subject:5"),
		tg.NewInlineKeyboardButtonData("انكليزي", "subject:6"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("اسلامية", "subject:7"),
		tg.NewInlineKeyboardButtonData("فرنسي", "subject:8"),
	),
)

var grade = tg.NewInlineKeyboardMarkup(
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("اول متوسط", "grade:1"),
		tg.NewInlineKeyboardButtonData("ثاني متوسط", "grade:2"),
		tg.NewInlineKeyboardButtonData("ثالث متوسط", "grade:3"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("رابع علمي", "grade:4"),
		tg.NewInlineKeyboardButtonData("خامس علمي", "grade:5"),
		tg.NewInlineKeyboardButtonData("سادس علمي", "grade:6"),
	),
)

var examMarkup = tg.NewInlineKeyboardMarkup(
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("الشهر الاول الفصل الاول", "exam:1"),
		tg.NewInlineKeyboardButtonData("الشهر الثاني الفصل الاول", "exam:2"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("نصف السنة", "exam:3"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("الشهر الاول الفصل الثاني", "exam:4"),
		tg.NewInlineKeyboardButtonData("الشهر الثاني الفصل الثاني", "exam:5"),
	),
	tg.NewInlineKeyboardRow(
		tg.NewInlineKeyboardButtonData("نهاية السنة", "exam:6"),
	),
)

func constructSubjectMarkup(grade int) [][]tg.InlineKeyboardButton {
	reply := subjectMarkup.InlineKeyboard
	switch grade {
	case 3:
		return append(reply, tg.NewInlineKeyboardRow(
			tg.NewInlineKeyboardButtonData("اجتماعيات", "subject:12"),
		))
	case 6:
		return reply
	case 1, 2, 4, 5:
		reply = append(reply, tg.NewInlineKeyboardRow(
			tg.NewInlineKeyboardButtonData("حاسوب", "subject:9"),
		))
		if grade == 4 {
			return append(reply, tg.NewInlineKeyboardRow(
				tg.NewInlineKeyboardButtonData("جرائم حزب البعث", "subject:10"),
			))
		}
		if grade == 1 || grade == 2 {
			reply = append(reply, tg.NewInlineKeyboardRow(
				tg.NewInlineKeyboardButtonData("التربية الاخلاقية", "subject:11"),
			))
			return append(reply, tg.NewInlineKeyboardRow(
				tg.NewInlineKeyboardButtonData("الاجتماعيات", "subject:12"),
			))
		}
		return reply
	}
	return reply
}

func constructDeleteMarkup(fileID string) tg.InlineKeyboardMarkup {
	return tg.NewInlineKeyboardMarkup(
		tg.NewInlineKeyboardRow(
			tg.NewInlineKeyboardButtonData("حذف الملف ", fmt.Sprintf("deleteImage:%s", fileID)),
		),
	)
}

func parseString(s string) (string, string) {
	parts := strings.SplitN(s, ":", 2)
	if len(parts) != 2 {
		return s, ""
	}
	return parts[0], parts[1]
}

func constructReplyMarkup(uploadID string) tg.InlineKeyboardMarkup {
	return tg.NewInlineKeyboardMarkup(
		tg.NewInlineKeyboardRow(
			tg.NewInlineKeyboardButtonData("موافق✅", fmt.Sprintf("confirmation:%s", uploadID)),
			tg.NewInlineKeyboardButtonData("غير موافق❌", fmt.Sprintf("disapproval:%s", uploadID)),
		),
	)
}

func (s *session) sessionReady() bool {
	return s != nil && s.Exam != 0 && s.Grade != 0 && s.Subject != 0 && s.Type != ""
}

func deleteCallbackMessage(cb tg.CallbackQuery) error {
	cfg := tg.DeleteMessageConfig{
		ChatID:    cb.Message.Chat.ID,
		MessageID: cb.Message.MessageID,
	}
	_, err := bot.Request(cfg)
	return err
}

func findFileIDs(s examConfig) ([]File, error) {
	rows, err := DB.Query(
		"SELECT id , fileID , title FROM exams WHERE exam = ? AND grade = ? AND subject = ?",
		s.Exam, s.Grade, s.Subject,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []File
	for rows.Next() {
		var f File

		if err := rows.Scan(&f.ID, &f.FileID, &f.Title); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, nil
}

func insertFile(p *pendingUpload) error {
	_, err := DB.Exec(
		"INSERT INTO exams (exam , grade , subject , title , fileID ) VALUES (? , ? , ? , ? , ?)",
		p.Exam, p.Grade, p.Subject, p.Title, p.FileID,
	)
	return err
}

func deleteImage(imageID string) error {

	_, err := DB.Exec("DELETE FROM exams WHERE id = ? ", imageID)

	return err
}

func getAdmins() ([]admin, error) {
	rows, err := DB.Query("SELECT tgID FROM admins")
	if err != nil {
		return nil, err
	}

	var admins []admin

	for rows.Next() {
		var currAdmin admin

		if err = rows.Scan(&currAdmin.TgID); err != nil {
			return nil, err
		}

		admins = append(admins, currAdmin)

	}

	return admins, nil

}

func isAdmin(userID int64) (bool, error) {

	admins, err := getAdmins()

	if err != nil {
		return false, err
	}

	for _, admin := range admins {
		if admin.TgID == strconv.FormatInt(userID, 10) {
			return true, nil
		}
	}
	return false, nil
}

func getNextAdmin() (admin, error) {
	admins, err := getAdmins()
	if err != nil {
		return admin{}, err
	}

	fmt.Println(len(admins))

	admin := admins[adminCounter]

	if adminCounter >= (len(admins) - 1) {
		adminCounter = 0
	} else {
		adminCounter++
	}

	return admin, nil
}

func main() {
	var err error
	token := os.Getenv("TOKEN")
	bot, err = tg.NewBotAPI(token)
	if err != nil {
		panic(err)
	}

	DB, err = sql.Open("sqlite3", "db/main.db")
	if err != nil {
		panic(err)
	}

	u := tg.NewUpdate(0)
	u.Timeout = 90
	updates := bot.GetUpdatesChan(u)

	fmt.Println("running")

	for update := range updates {

		if update.Message != nil {
			msg := update.Message
			newMsg := tg.NewMessage(msg.Chat.ID, "")

			if msg.Photo != nil {
				s, exists := sessions[msg.From.ID]
				if !exists || !s.sessionReady() || s.Title == "" {
					newMsg.Text = "يجب ملئ كل المعلومات حتى ترسل الصورة"
					bot.Send(newMsg)
					continue
				}

				photo := msg.Photo[len(msg.Photo)-1].FileID
				uploadID := fmt.Sprintf("%d_%d", msg.From.ID, time.Now().UnixNano())

				isAdmin, err := isAdmin(update.Message.From.ID)

				if err != nil {
					fmt.Println(err)
				}

				pendingUpload := &pendingUpload{
					ID:      uploadID,
					UserID:  msg.From.ID,
					Grade:   s.Grade,
					Exam:    s.Exam,
					Subject: s.Subject,
					Title:   s.Title,
					FileID:  photo,
					ChatID:  msg.Chat.ID,
				}

				if isAdmin {

					if err := insertFile(pendingUpload); err == nil {
						success := tg.NewMessage(pendingUpload.ChatID, "تم اضافة الملف بنجاح")
						bot.Send(success)
					}
					continue
				}

				pendingUploads[uploadID] = pendingUpload

				newMsg.Text = "سوف يتم الارسال الى المشرفين لتأكيد الاضافة"
				bot.Send(newMsg)

				admin, err := getNextAdmin()
				if err != nil {
					fmt.Println(err)
					continue
				}

				tgID, err := strconv.ParseInt(admin.TgID, 10, 64)

				if err != nil {
					fmt.Println(err)
					continue
				}

				f := tg.NewForward(tgID, msg.Chat.ID, msg.MessageID)
				bot.Send(f)

				ask := tg.NewMessage(tgID, "هل توافق")
				ask.ReplyMarkup = constructReplyMarkup(uploadID)
				bot.Send(ask)

				continue
			}

			if msg.Command() == "start" {
				start := tg.NewMessage(msg.Chat.ID, "الاوامر : ")
				start.ReplyMarkup = startMarkup
				bot.Send(start)
				continue
			}

			if s, exists := sessions[msg.From.ID]; exists && s.LookingForTitle {
				s.LookingForTitle = false
				s.Title = msg.Text
				newMsg.Text = "ارسل صور الامتحان "
				bot.Send(newMsg)
			}
		}

		if update.CallbackQuery != nil {
			cb := update.CallbackQuery
			action, value := parseString(cb.Data)
			newMsg := tg.NewMessage(cb.Message.Chat.ID, "")

			switch action {

			case "uploadExam", "showExam":
				delete(sessions, cb.From.ID)
				sessions[cb.From.ID] = &session{Type: action}
				newMsg.Text = "حدد الصف"
				newMsg.ReplyMarkup = grade
				bot.Send(newMsg)
				deleteCallbackMessage(*cb)

			case "grade":
				s, exists := sessions[cb.From.ID]
				if !exists {
					newMsg.Text = "يرجى اعادة ملئ المعلومات"
					newMsg.ReplyMarkup = startMarkup
					bot.Send(newMsg)
					continue
				}
				g, _ := strconv.Atoi(value)
				s.Grade = g
				newMsg.Text = "اختار الامتحان"
				newMsg.ReplyMarkup = examMarkup
				bot.Send(newMsg)
				deleteCallbackMessage(*cb)

			case "exam":
				s, exists := sessions[cb.From.ID]
				if !exists {
					continue
				}
				e, _ := strconv.Atoi(value)
				s.Exam = e
				newMsg.Text = "اختار المادة"
				newMsg.ReplyMarkup = tg.InlineKeyboardMarkup{
					InlineKeyboard: constructSubjectMarkup(s.Grade),
				}
				bot.Send(newMsg)
				deleteCallbackMessage(*cb)

			case "subject":
				s, exists := sessions[cb.From.ID]
				if !exists {
					continue
				}
				sub, _ := strconv.Atoi(value)
				s.Subject = sub
				deleteCallbackMessage(*cb)

				if s.Type == "uploadExam" {
					if !s.sessionReady() {
						continue
					}
					s.LookingForTitle = true
					newMsg.Text = "اكتب معلومات اضافية للامتحان (مثل مدرس المادة و السنة )"
					bot.Send(newMsg)
				}

				if s.Type == "showExam" {
					files, err := findFileIDs(examConfig{
						Exam:    s.Exam,
						Grade:   s.Grade,
						Subject: s.Subject,
					})
					if err != nil || len(files) == 0 {
						newMsg.Text = "لا يوجد نماذج لهذا الامتحان حاليا"
						bot.Send(newMsg)
						delete(sessions, cb.From.ID)
						continue
					}
					isAdmin, err := isAdmin(cb.From.ID)
					if err != nil {
						panic(err)
					}
					for _, f := range files {
						photo := tg.NewPhoto(cb.From.ID, tg.FileID(f.FileID))
						photo.Caption = f.Title
						if isAdmin {
							fmt.Println("yes he is an admin indeed")

							photo.ReplyMarkup = constructDeleteMarkup(f.ID)
						}
						bot.Send(photo)

					}

					delete(sessions, cb.From.ID)
				}

			case "confirmation":
				p, exists := pendingUploads[value]
				if !exists {
					bot.Send(tg.NewMessage(cb.From.ID, "تم القبول سابقا"))
					continue
				}
				if err := insertFile(p); err == nil {
					success := tg.NewMessage(p.ChatID, "تم اضافة الملف بنجاح")
					bot.Send(success)
					success.ChatID = cb.From.ID
					bot.Send(success)
				}
				delete(pendingUploads, value)

			case "disapproval":
				p, exists := pendingUploads[value]
				if !exists {
					continue
				}
				reject := tg.NewMessage(p.ChatID, "تم رفض اضافة الملف")
				bot.Send(reject)
				delete(pendingUploads, value)
			case "deleteImage":
				chatID := cb.From.ID
				isAdmin, err := isAdmin(chatID)
				if err != nil {
					panic(err)
				}
				if !isAdmin {
					fmt.Println("not authorized !")
					bot.Send(tg.NewMessage(cb.Message.Chat.ID, "انت لست مشرف"))
					continue
				}

				err = deleteImage(value)
				if err != nil {
					if err == sql.ErrNoRows {
						if _, err := bot.Send(tg.NewMessage(cb.From.ID, "هذه الصورة غير موجودة")); err != nil {
							fmt.Print(err)
						}

						continue
					}

					fmt.Println(err)
					continue
				}

				bot.Send(tg.NewMessage(cb.From.ID, "تم حذف الملف بنجاح"))

			}
		}
	}
}
