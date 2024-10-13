package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Transaction struct {
	Date        string  `json:"date"`
	Description string  `json:"description"`
	IsExpense   bool    `json:"isExpense"`
	Number      float64 `json:"number"`
	Balance     float64 `json:"balance"`
}

var days_full = []string{
	"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
	"11", "12", "13", "14", "15", "16", "17", "18", "19", "20",
	"21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31",
}

// var uri = "mongodb://leaf:mongodbroot@127.0.0.1/admin"
var uri string

var mongo_client *mongo.Client

var database *mongo.Database

var yearDropdown, monthDropdown, dayDropdown *widget.Select

var descriptionEntry, amountEntry *widget.Entry

var expenseCheckbox *widget.Check

var submitButton *widget.Button

var infomation *widget.Entry

var balance, tempBalance float64

var balance_file_path string

var address = "127.0.0.1"

var user = "leaf"

var password = "mongodbroot"

var login_db = "admin"

var reayForSubmit *Transaction

func main() {

	initData()

	// 创建应用程序
	myApp := app.New()
	myApp.Settings().SetTheme(theme.LightTheme())

	// 创建主窗口
	myWindow := myApp.NewWindow("收支登记GUI版")

	// 创建下拉列表
	yearDropdown = widget.NewSelect([]string{"2020", "2021", "2022", "2023", "2024", "2025", "2026", "2027", "2028", "2029"}, nil)
	dayDropdown = widget.NewSelect(days_full, nil)
	monthDropdown = widget.NewSelect(days_full[:12], func(selected string) {
		switch selected {
		case "1", "3", "5", "7", "8", "10", "12":
			dayDropdown.SetOptions(days_full)
		case "4", "6", "9", "11":
			dayDropdown.SetOptions(days_full[:30])
		case "2":
			year, _ := strconv.Atoi(yearDropdown.Selected)
			// 该软件编写于2024年，开发者认为活不到2100年，故此处平年闰年的判断仅按4的倍数来，不管是否400的倍数
			if year%4 == 0 {
				dayDropdown.SetOptions(days_full[:29])
				break
			}
			dayDropdown.SetOptions(days_full[:28])
		}
	})
	yearDropdown.Selected = strconv.Itoa(time.Now().Year())
	monthDropdown.Selected = strconv.Itoa(int(time.Now().Month()))
	dayDropdown.Selected = strconv.Itoa(time.Now().Day())

	// 创建文本框和标签
	descriptionEntry = widget.NewEntry()
	// descriptionEntry.SetPlaceholder("描述")
	descriptionEntry.SetPlaceHolder("描述")
	expenseCheckbox = widget.NewCheck("支出", func(bool) {})
	expenseCheckbox.SetChecked(true)

	amountEntry = widget.NewEntry()
	amountEntry.SetPlaceHolder("仅限数字且为正数")

	// 创建提交按钮
	submitButton = widget.NewButton("提交", func() {
		// balance = tempBalance
		//提交

		year := yearDropdown.Selected
		month := monthDropdown.Selected
		day := dayDropdown.Selected
		description := descriptionEntry.Text
		isExpense := expenseCheckbox.Checked
		amount, _ := strconv.ParseFloat(amountEntry.Text, 64)
		if isExpense {
			tempBalance = balance - amount
		} else {
			tempBalance = balance + amount
		}
		tmp := strconv.FormatFloat(tempBalance, 'f', 2, 64)
		tempBalance, _ = strconv.ParseFloat(tmp, 64)
		// 啥比golang，居然不支持三目运算法 really?true:false
		reayForSubmit = &Transaction{year + "-" + month + "-" + day, description, isExpense, amount, tempBalance}
		infomation.SetText(transactionToJsonObj(*reayForSubmit))
		dialog.ShowConfirm("确认提交?", transactionToJsonObj(*reayForSubmit), func(b bool) {
			if b {
				submitButton.Disable()
				go postTransaction("a"+year+"_"+month, *reayForSubmit)
			}
		}, myWindow)
		// submitButton.Disable()

	})
	// submitButton.Disable()

	// infomation = widget.NewLabel("未操作")
	infomation = widget.NewMultiLineEntry()
	infomation.SetText("未操作")
	infomation.Wrapping = fyne.TextWrapWord

	initMongo()

	content := container.New(layout.NewVBoxLayout(), container.New(layout.NewHBoxLayout(), yearDropdown, monthDropdown, dayDropdown),
		descriptionEntry,
		expenseCheckbox,
		container.New(layout.NewGridLayout(2), amountEntry, widget.NewLabel("金额")),
		// previewButton,
		submitButton,
		infomation,
	)

	// 设置窗口内容和大小
	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(400, 300))

	myWindow.SetOnClosed(func() {
		// 写出余额信息
		conf, err := os.OpenFile(balance_file_path, os.O_WRONLY, 0)
		if err != nil {
			panic(err)
		}
		n, err := conf.WriteString(strconv.FormatFloat(balance, 'f', 2, 64))
		conf.Close()
		if err != nil {
			fmt.Println(n)
			panic(err)
		}
		// 断开mongo的连接
		err = mongo_client.Disconnect(context.TODO())
		if err != nil {
			fmt.Println(n)
			panic(err)
		}
	})

	// 显示窗口
	myWindow.ShowAndRun()

}

func initMongo() {
	uri = "mongodb://" + user + ":" + password + "@" + address + "/" + login_db
	var err error
	mongo_client, err = mongo.Connect(context.TODO(), options.Client().ApplyURI(uri))
	if err != nil {
		infomation.SetText("连接数据库失败")
		return
	}
	database = mongo_client.Database("deposit")
	infomation.SetText("mongodb 初始化完毕。数据库名" + database.Name())
}

func initData() {
	//读文件balance_file
	//如果没有，亖
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exec_path := filepath.Dir(ex)
	fmt.Println("路径:", exec_path)
	balance_file_path = exec_path + "/balance_file"
	balance_file, err := os.Open(balance_file_path)
	if err != nil {
		panic("无balance_file文件")
	}
	defer balance_file.Close() // 确保在函数结束时关闭文件
	scanner := bufio.NewScanner(balance_file)
	if scanner.Scan() {
		balance_string := scanner.Text()
		balance, err = strconv.ParseFloat(balance_string, 64)
		if err != nil {
			panic("你几把在文件里写了什么东西? ")
		}
	} else {
		panic("为什么balance_file文件是空的?")
	}

	// 读文件config
	// 第一行是ip(或加端口)
	// 第二行是用户名
	// 第三行是密码
	// 第四行是登录数据库
	config_file, err := os.Open(exec_path + "/config")
	if err != nil {
		fmt.Println("找不到配置文件config", err)
		return
	}
	defer config_file.Close() // 确保在函数结束时关闭文件

	// 创建一个新的bufio.Scanner对象
	scanner = bufio.NewScanner(config_file)
	if scanner.Scan() {
		address = scanner.Text()
	}
	if scanner.Scan() {
		user = scanner.Text()
	}
	if scanner.Scan() {
		password = scanner.Text()
	}
	if scanner.Scan() {
		login_db = scanner.Text()
	}

	// 检查是否有扫描时产生的错误
	if err := scanner.Err(); err != nil {
		fmt.Println("Error reading file:", err)
	}
}

func transactionToJsonObj(transaction_item Transaction) string {
	v, err := json.Marshal(transaction_item)
	if err != nil {
		panic(err)
	}
	return string(v)
}

func postTransaction(collection_name string, item Transaction) {

	collection := database.Collection(collection_name)
	fmt.Println("提交中 表名", collection.Name())
	result, err := collection.InsertOne(context.TODO(), item)
	if err != nil {
		submitCallback("提交失败，原因见控制台")
		fmt.Println(err)
		return
	}
	fmt.Println(result.InsertedID)
	submitCallback(fmt.Sprintf("%s", result.InsertedID))
	clearCallback()
	balance = item.Balance
}
func submitCallback(info string) {
	infomation.SetText(info)
	submitButton.Enable()
}
func clearCallback() {
	descriptionEntry.SetText("")
	amountEntry.SetText("")
}
