package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/mitchellh/go-homedir"
	"github.com/tzmfreedom/goroon"
	"github.com/urfave/cli"
)

var (
	Version  string
	Revision string
)

type config struct {
	Username string
	Password string
	Endpoint string
	UserId   string
	Debug    bool
	Start    string
	End      string
	TopicId  int
	Offset   int
	Limit    int
	Type     string
	Columns  string
	Date     string
}

type configFile struct {
	SessionId string
	Endpoint  string
}

func main() {
	cli.VersionPrinter = func(c *cli.Context) {
		fmt.Printf("version=%s revision=%s\n", c.App.Version, Revision)
	}

	c := &config{}
	app := cli.NewApp()
	app.Name = "goroon"
	app.Usage = "garoon utility"
	app.Version = Version
	app.Commands = []cli.Command{
		{
			Name:    "login",
			Aliases: []string{"l"},
			Usage:   "login to garoon",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "username, u",
					Destination: &c.Username,
					EnvVar:      "GAROON_USERNAME",
				},
				cli.StringFlag{
					Name:        "password, p",
					Destination: &c.Password,
					EnvVar:      "GAROON_PASSWORD",
				},
				cli.StringFlag{
					Name:        "endpoint, e",
					Destination: &c.Endpoint,
					EnvVar:      "GAROON_ENDPOINT",
				},
				cli.BoolFlag{
					Name:        "debug, D",
					Destination: &c.Debug,
				},
			},
			Action: func(ctx *cli.Context) error {
				client := goroon.NewClient(c.Endpoint)
				if c.Debug {
					client.Debugger = os.Stdout
				}
				res, err := client.UtilLogin(&goroon.Parameters{
					LoginName: []string{c.Username},
					Password:  c.Password,
				})
				if err != nil {
					return err
				}
				r := regexp.MustCompile(client.SessionKey + `=(.+?);`)
				group := r.FindAllStringSubmatch(res.Cookie, -1)
				createConfigFile(group[0][1], c.Endpoint)
				return err
			},
		},
		{
			Name:    "schedule",
			Aliases: []string{"s"},
			Usage:   "get your schedule",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "username, u",
					Destination: &c.Username,
					EnvVar:      "GAROON_USERNAME",
				},
				cli.StringFlag{
					Name:        "password, p",
					Destination: &c.Password,
					EnvVar:      "GAROON_PASSWORD",
				},
				cli.StringFlag{
					Name:        "endpoint, e",
					Destination: &c.Endpoint,
					EnvVar:      "GAROON_ENDPOINT",
				},
				cli.StringFlag{
					Name:        "userid, i",
					Destination: &c.UserId,
				},
				cli.StringFlag{
					Name:        "start",
					Destination: &c.Start,
				},
				cli.StringFlag{
					Name:        "end",
					Destination: &c.End,
				},
				cli.StringFlag{
					Name:        "type, t",
					Destination: &c.Type,
					Value:       "all",
				},
				cli.StringFlag{
					Name:        "columns, c",
					Destination: &c.Columns,
					Value:       "detail,start,end",
				},
				cli.BoolFlag{
					Name:        "debug, D",
					Destination: &c.Debug,
				},
				cli.StringFlag{
					Name:        "date, d",
					Destination: &c.Date,
				},
			},
			Action: func(ctx *cli.Context) error {
				client := newGaroonClient(c)
				var (
					start time.Time
					end   time.Time
					err   error
				)
				switch c.Date {
				case "":
					if c.Start != "" {
						start, err = time.ParseInLocation("2006-01-02 15:04:05", c.Start, time.Local)
						if err != nil {
							return err
						}
					} else {
						start = beginningOfDay(time.Now())
					}
					if c.End != "" {
						end, err = time.ParseInLocation("2006-01-02 15:04:05", c.End, time.Local)
						if err != nil {
							return err
						}
					} else {
						end = endOfDay(time.Now())
					}
				case "today":
					now := time.Now()
					start = beginningOfDay(now)
					end = endOfDay(now)
				case "yesterday":
					yesterday := time.Now().AddDate(0, 0, -1)
					start = beginningOfDay(yesterday)
					end = endOfDay(yesterday)
				}

				var returns *goroon.Returns
				if c.UserId != "" {
					res, err := client.BaseGetUserByLoginName(&goroon.Parameters{
						LoginName: []string{c.UserId},
					})
					if err != nil {
						return err
					}
					returns, err = client.ScheduleGetEventsByTarget(&goroon.Parameters{
						Start: goroon.XmlDateTime{start.In(time.UTC)},
						End:   goroon.XmlDateTime{end.In(time.UTC)},
						User: goroon.User{
							Id: res.UserId,
						},
					})
					if err != nil {
						return err
					}
				} else {
					returns, err = client.ScheduleGetEvents(&goroon.Parameters{
						Start: goroon.XmlDateTime{start.In(time.UTC)},
						End:   goroon.XmlDateTime{end.In(time.UTC)},
					})
					if err != nil {
						return err
					}
				}

				print_cols := strings.Split(c.Columns, ",")
				for _, event := range returns.ScheduleEvents {
					if c.Type != "all" && event.EventType != c.Type {
						continue
					}
					printScheduleEvent(&event, print_cols)
				}
				return nil
			},
		},
		{
			Name:    "bulletin",
			Aliases: []string{"b"},
			Usage:   "get bulletin",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "username, u",
					Destination: &c.Username,
					EnvVar:      "GAROON_USERNAME",
				},
				cli.StringFlag{
					Name:        "password, p",
					Destination: &c.Password,
					EnvVar:      "GAROON_PASSWORD",
				},
				cli.StringFlag{
					Name:        "endpoint, e",
					Destination: &c.Endpoint,
					EnvVar:      "GAROON_ENDPOINT",
				},
				cli.IntFlag{
					Name:        "topic_id",
					Destination: &c.TopicId,
				},
				cli.IntFlag{
					Name:        "offset, o",
					Destination: &c.Offset,
				},
				cli.IntFlag{
					Name:        "limit, l",
					Destination: &c.Limit,
					Value:       20,
				},
				cli.BoolFlag{
					Name:        "debug, D",
					Destination: &c.Debug,
				},
				cli.StringFlag{
					Name:        "columns, c",
					Destination: &c.Columns,
					Value:       "creator,text",
				},
			},
			Action: func(ctx *cli.Context) error {
				client := newGaroonClient(c)
				res, err := client.BulletinGetFollows(&goroon.Parameters{
					TopicId: c.TopicId,
					Offset:  c.Offset,
					Limit:   c.Limit,
				})
				if err != nil {
					return err
				}
				print_cols := strings.Split(c.Columns, ",")
				for _, follow := range res.Follow {
					printFollow(&follow, print_cols)
				}
				return nil
			},
		},
	}
	app.Run(os.Args)
}

func startStr(event *goroon.ScheduleEvent) string {
	if event.EventType == "repeat" {
		return fmt.Sprintf("%sT%s", time.Now().Format("2006-01-02"), event.RepeatInfo.Condition.StartTime)
	}
	if isNullTime(event.When.Datetime.Start) {
		return formatDate(event.When.Date.Start)
	}
	return formatDatetime(event.When.Datetime.Start)
}

func endStr(event *goroon.ScheduleEvent) string {
	if event.EventType == "repeat" {
		return fmt.Sprintf("%sT%s", time.Now().Format("2006-01-02"), event.RepeatInfo.Condition.EndTime)
	}
	if isNullTime(event.When.Datetime.End) {
		return formatDate(event.When.Date.End)
	}
	return formatDatetime(event.When.Datetime.End)
}

func members2str(members *goroon.Members) string {
	ret := []string{}
	for _, m := range members.Member {
		ret = append(ret, m.User.Name)
	}
	return strings.Join(ret, ":")
}

func isNullTime(t time.Time) bool {
	var nil time.Time
	return t == nil
}

func formatDate(t goroon.XmlDate) string {
	return t.Format("2006-01-02")
}

func formatDatetime(t time.Time) string {
	return t.In(time.Local).Format("2006-01-02T15:04:05")
}

func readConfigFile() (*configFile, error) {
	home, err := homedir.Dir()
	if err != nil {
		return nil, err
	}
	b, err := ioutil.ReadFile(filepath.Join(home, ".goroon"))
	if err != nil {
		return nil, err
	}
	cf := &configFile{}
	err = toml.Unmarshal(b, cf)
	return cf, err
}

func newGaroonClient(c *config) *goroon.Client {
	var client *goroon.Client
	cf, err := readConfigFile()
	if err == nil {
		client = goroon.NewClient(cf.Endpoint)
		client.SessionId = cf.SessionId
	} else {
		client = goroon.NewClient(c.Endpoint)
		client.Username = c.Username
		client.Password = c.Password
	}
	if c.Debug {
		client.Debugger = os.Stdout
	}
	return client
}

func printScheduleEvent(e *goroon.ScheduleEvent, cols []string) {
	print_cols := []string{}
	for _, col := range cols {
		print_col := ""
		switch col {
		case "id":
			print_col = fmt.Sprint(e.Id)
		case "members":
			print_col = members2str(&e.Members)
		case "type":
			print_col = e.EventType
		case "detail":
			print_col = strings.Replace(e.Detail, "\n", "", -1)
		case "desc":
			print_col = strings.Replace(e.Description, "\n", "", -1)
		case "start":
			print_col = startStr(e)
		case "end":
			print_col = endStr(e)
		}
		print_cols = append(print_cols, print_col)
	}
	fmt.Println(strings.Join(print_cols, "\t"))
}

func printFollow(f *goroon.Follow, cols []string) {
	print_cols := []string{}
	for _, col := range cols {
		print_col := ""
		switch col {
		case "id":
			print_col = fmt.Sprint(f.Number)
		case "creator":
			print_col = f.Creator.Name
		case "text":
			print_col = strings.Replace(f.Text, "\r\n", " ", -1)
		}
		print_cols = append(print_cols, print_col)
	}
	fmt.Println(strings.Join(print_cols, "\t"))
}

func createConfigFile(sessionId string, endpoint string) error {
	home, err := homedir.Dir()
	if err != nil {
		return err
	}
	f, err := os.Create(filepath.Join(home, ".goroon"))
	if err != nil {
		return err
	}
	e := toml.NewEncoder(f)
	err = e.Encode(&configFile{
		SessionId: sessionId,
		Endpoint:  endpoint,
	})
	return err
}

func beginningOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, t.Location())
}

func endOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 23, 59, 59, 999999, t.Location())
}
