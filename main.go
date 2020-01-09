package main

import (
	//"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"strings"
	"sync"

	//外部ファイル
	"github.com/gin-gonic/gin"
	_ "github.com/denisenkom/go-mssqldb"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
)

type User struct {
	UserId   string `json:"userid"`
	Password string `json:"password"`
}

type VarChart struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PieChart struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type Param struct {
	SqlTarget string `json:"sqltarget"`
	SqlTerm   string `json:"sqlterm"`
	SqlTime   string `json:"sqltime"`
}

type ReturnTest struct {
	Id string `json:"id"`
	Name string `json:"name"`
	OverTime string `json:"overtime"`
}

 type SessionInfo struct {
 	UserId         interface{}  //ログインしているユーザのID
// 	IsSessionAlive bool        //セッションが生きているかどうか
}

var Info SessionInfo

func CORSMiddleware() gin.HandlerFunc {
	return func(g *gin.Context) {
		g.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		g.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type/json, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		g.Writer.Header().Set("Access-Control-Allow-Methods", "DELETE, GET, OPTIONS, POST, PUT")

		if g.Request.Method == "OPTIONS" {
			g.AbortWithStatus(204)
			return
		}

		g.Next()
	}
}

func main() {
	//おまじない
	server := gin.Default()
	store := cookie.NewStore([]byte("secret"))
	server.Use(sessions.Sessions("mysession", store))

	server.Use(CORSMiddleware())
	
	//Action
	server.GET("/getBarChartData", getBarChartData)
	server.GET("/getPieChartData", getPieChartData)
	
	server.POST("/getListData", getListData)
	server.POST("/impcsv", impcsv)
	server.POST("/sessionCheck", sessionCheck)
	server.POST("/signin", signin)

	//指定のポート番号でサーバー実行
	server.Run(":8888")
}

func sessionCheck(g *gin.Context) {
	g.BindJSON(&Info)
	
	// // セッションがない場合
	// if Info.UserId == nil {
	// 	g.JSON(440,"セッションが切れています。ログインしなおしてください。")
	// // セッションがある場合
	// } else {
		g.JSON(200, Info)
	// }

}

func signin(g *gin.Context) {

	//構造体定義
	var user User
	var cnt int

	//dbは*sql.DB
	db, openerr := sql.Open("sqlserver", getconnString("ConnectionString.txt"))
    if openerr != nil {
        log.Fatal("Error creating connection pool: ", openerr.Error())
    }

	//ログインユーザー用変数の変数にバインド
	g.BindJSON(&user)


	// Select文発行
	rows,queryerr := db.Query(
		getSQL("S-signin.txt"),
	    sql.NamedArg{ Name: "UserID", Value:  user.UserId },
	    sql.NamedArg{ Name: "Password", Value:  user.Password })
	// エラーがない場合、セッションセット
	if queryerr == nil {
		session := sessions.Default(g)
		session.Options(sessions.Options{MaxAge: 600})
		session.Set("UserId", user.UserId)
		session.Save()

		rows.Next()
		queryerr = rows.Scan(&cnt)

		//JSONにして返却
		g.JSON(200, gin.H{"UserID": user.UserId, "usercnt":cnt})
	// エラーの場合
	}else{
		log.Fatal("Error creating connection pool#: ", queryerr)
	}
}

func impcsv(g *gin.Context) {

	////DnDされたファイルを受け取る
	receivefile, header, err := g.Request.FormFile("upload")

	// ファイル名
	filename := header.Filename

	// ファイルをtmpフォルダに保存する
	out, err := os.Create("./tmp/" + filename)
	if err != nil {
		log.Fatal(err)
	}

	//必ずclose＠おまじない
	defer out.Close()
	//DnDされたファイルをサーバーにコピー
	_, err = io.Copy(out, receivefile)
	if err != nil {
		log.Fatal(err)
	}

	//コピーしたファイルをOPEN
	file, err := os.Open("./tmp/" + filename)
	if err != nil {
		fmt.Println("Error", err)
		return
	}

	//必ずclose＠おまじない
	defer file.Close()

	//CSV読み込み
	reader := csv.NewReader(file)
	// 全部メモリに読み込み
	record, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error", err)
	}

	//エラーチェック
	errnum := errcheck(filename, record)
	if errnum > 0 {
		// 掴みっぱだとファイルが消せないのでclose
		out.Close()
		file.Close()

		// エラー番号別処理
		switch errnum {
		// 1:ファイル拡張子エラー
		case 1:
			g.JSON(400, "CSVファイルを指定してください")

		// 2:項目エラー
		case 2:
			g.JSON(400, "項目数が足りません。")

		}

		err := os.Remove("./tmp/" + filename)
		if err != nil {
			log.Fatal(err)
		}

		// 戻る
		return
	}

	//insert実行
	execInsert(record)
}

func errcheck(filename string, record [][]string) int {

	// 拡張子チェック
	pos := strings.LastIndex(filename, ".")
	extension := strings.ToUpper(filename[pos:])
	if extension != ".CSV" {
		return 1
	}

	// 項目数チェック
	for value := range record {
		if len(record[value]) <= 85 {
			return 2
		}
	}

	//何も問題なければ0
	return 0
}

func execInsert(record [][]string) {
	// コネクション作成
	//dbは*sql.DB
	db, openerr := sql.Open("sqlserver", getconnString("ConnectionString.txt"))
    if openerr != nil {
        log.Fatal("Error creating connection pool: ", openerr.Error())
    }
	// 必ずclose＠おまじない
	defer db.Close()

	// トランザクションスタート
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}

	//複数のgoroutineの完了を待つための準備
	var wg sync.WaitGroup

	// SQL実行 CSVファイルの行数分繰り返し
	for value := range record { // for i:=0; i<len(record)

		// 位置を変数にコピー(直接使い回すのはダメらしい)
		i := value

		// WaitGroupに1つ追加(インクリメント)
		wg.Add(1)

		// goroutineで並列処理
		go func() {

			// 最後にWaitGroupを終了(デクリメント)
			defer wg.Done()

			// Insert文発行
			_, err = tx.Exec(
				getSQL("I-execInsert.txt"),
				sql.NamedArg{ Name: "P0", Value: record[i][0]},
				sql.NamedArg{ Name: "P1", Value: record[i][1]},
				sql.NamedArg{ Name: "P2", Value: record[i][2]},
				sql.NamedArg{ Name: "P3", Value: record[i][3]},
				sql.NamedArg{ Name: "P4", Value: record[i][4]},
				sql.NamedArg{ Name: "P5", Value: record[i][5]},
				sql.NamedArg{ Name: "P6", Value: record[i][6]},
				sql.NamedArg{ Name: "P7", Value: record[i][7]},
				sql.NamedArg{ Name: "P8", Value: record[i][8]},
				sql.NamedArg{ Name: "P9", Value: record[i][9]},
				sql.NamedArg{ Name: "P10", Value: record[i][10]},
				sql.NamedArg{ Name: "P11", Value: record[i][11]},
				sql.NamedArg{ Name: "P12", Value: record[i][12]},
				sql.NamedArg{ Name: "P13", Value: record[i][13]},
				sql.NamedArg{ Name: "P14", Value: record[i][14]},
				sql.NamedArg{ Name: "P15", Value: record[i][15]},
				sql.NamedArg{ Name: "P16", Value: record[i][16]},
				sql.NamedArg{ Name: "P17", Value: record[i][17]},
				sql.NamedArg{ Name: "P18", Value: record[i][18]},
				sql.NamedArg{ Name: "P19", Value: record[i][19]},
				sql.NamedArg{ Name: "P20", Value: record[i][20]},
				sql.NamedArg{ Name: "P21", Value: record[i][21]},
				sql.NamedArg{ Name: "P22", Value: record[i][22]},
				sql.NamedArg{ Name: "P23", Value: record[i][23]},
				sql.NamedArg{ Name: "P24", Value: record[i][24]},
				sql.NamedArg{ Name: "P25", Value: record[i][25]},
				sql.NamedArg{ Name: "P26", Value: record[i][26]},
				sql.NamedArg{ Name: "P27", Value: record[i][27]},
				sql.NamedArg{ Name: "P28", Value: record[i][28]},
				sql.NamedArg{ Name: "P29", Value: record[i][29]},
				sql.NamedArg{ Name: "P30", Value: record[i][30]},
				sql.NamedArg{ Name: "P31", Value: record[i][31]},
				sql.NamedArg{ Name: "P32", Value: record[i][32]},
				sql.NamedArg{ Name: "P33", Value: record[i][33]},
				sql.NamedArg{ Name: "P34", Value: record[i][34]},
				sql.NamedArg{ Name: "P35", Value: record[i][35]},
				sql.NamedArg{ Name: "P36", Value: record[i][36]},
				sql.NamedArg{ Name: "P37", Value: record[i][37]},
				sql.NamedArg{ Name: "P38", Value: record[i][38]},
				sql.NamedArg{ Name: "P39", Value: record[i][39]},
				sql.NamedArg{ Name: "P40", Value: record[i][40]},
				sql.NamedArg{ Name: "P41", Value: record[i][41]},
				sql.NamedArg{ Name: "P42", Value: record[i][42]},
				sql.NamedArg{ Name: "P43", Value: record[i][43]},
				sql.NamedArg{ Name: "P44", Value: record[i][44]},
				sql.NamedArg{ Name: "P45", Value: record[i][45]},
				sql.NamedArg{ Name: "P46", Value: record[i][46]},
				sql.NamedArg{ Name: "P47", Value: record[i][47]},
				sql.NamedArg{ Name: "P48", Value: record[i][48]},
				sql.NamedArg{ Name: "P49", Value: record[i][49]},
				sql.NamedArg{ Name: "P50", Value: record[i][50]},
				sql.NamedArg{ Name: "P51", Value: record[i][51]},
				sql.NamedArg{ Name: "P52", Value: record[i][52]},
				sql.NamedArg{ Name: "P53", Value: record[i][53]},
				sql.NamedArg{ Name: "P54", Value: record[i][54]},
				sql.NamedArg{ Name: "P55", Value: record[i][55]},
				sql.NamedArg{ Name: "P56", Value: record[i][56]},
				sql.NamedArg{ Name: "P57", Value: record[i][57]},
				sql.NamedArg{ Name: "P58", Value: record[i][58]},
				sql.NamedArg{ Name: "P59", Value: record[i][59]},
				sql.NamedArg{ Name: "P60", Value: record[i][60]},
				sql.NamedArg{ Name: "P61", Value: record[i][61]},
				sql.NamedArg{ Name: "P62", Value: record[i][62]},
				sql.NamedArg{ Name: "P63", Value: record[i][63]},
				sql.NamedArg{ Name: "P64", Value: record[i][64]},
				sql.NamedArg{ Name: "P65", Value: record[i][65]},
				sql.NamedArg{ Name: "P66", Value: record[i][66]},
				sql.NamedArg{ Name: "P67", Value: record[i][67]},
				sql.NamedArg{ Name: "P68", Value: record[i][68]},
				sql.NamedArg{ Name: "P69", Value: record[i][69]},
				sql.NamedArg{ Name: "P70", Value: record[i][70]},
				sql.NamedArg{ Name: "P71", Value: record[i][71]},
				sql.NamedArg{ Name: "P72", Value: record[i][72]},
				sql.NamedArg{ Name: "P73", Value: record[i][73]},
				sql.NamedArg{ Name: "P74", Value: record[i][74]},
				sql.NamedArg{ Name: "P75", Value: record[i][75]},
				sql.NamedArg{ Name: "P76", Value: record[i][76]},
				sql.NamedArg{ Name: "P77", Value: record[i][77]},
				sql.NamedArg{ Name: "P78", Value: record[i][78]},
				sql.NamedArg{ Name: "P79", Value: record[i][79]},
				sql.NamedArg{ Name: "P80", Value: record[i][80]},
				sql.NamedArg{ Name: "P81", Value: record[i][81]},
				sql.NamedArg{ Name: "P82", Value: record[i][82]},
				sql.NamedArg{ Name: "P83", Value: record[i][83]},
				sql.NamedArg{ Name: "P84", Value: record[i][84]},
				sql.NamedArg{ Name: "P85", Value: record[i][85]},
				sql.NamedArg{ Name: "P86", Value: record[i][86]},
				sql.NamedArg{ Name: "P87", Value: record[i][87]},
				sql.NamedArg{ Name: "P88", Value: record[i][88]},
				sql.NamedArg{ Name: "P89", Value: record[i][89]},
				sql.NamedArg{ Name: "P90", Value: record[i][90]},
				sql.NamedArg{ Name: "P91", Value: record[i][91]},
				sql.NamedArg{ Name: "P92", Value: record[i][92]},
				sql.NamedArg{ Name: "P93", Value: record[i][93]},
			)

			if err != nil {
				tx.Rollback()
				panic(err.Error())
			}

		}()
		wg.Wait()
	}

	//コミット
	tx.Commit()
}

func getBarChartData(g *gin.Context) {

	//返却用構造体
	var varchart []VarChart
	//scan用変数
	var name, value string

	// コネクション作成
	db, err := sql.Open("sqlserver", getconnString("ConnectionString.txt"))
	if err != nil {
		panic(err.Error())
	}
	// 必ずclose＠おまじない
	defer db.Close()

	//SQL実行
	rows, err := db.Query(getSQL("S-getBarChartData.txt"))
	if err != nil {
		panic(err.Error())
	}

	//取得値をscanして構造体に格納
	for rows.Next() {
		strange := rows.Scan(&name, &value)
		if strange != nil {
			log.Fatal(strange)
		}
		varchart = append(varchart, VarChart{Name: name, Value: value})
	}

	//JSONにして返却
	g.JSON(200, varchart)

	return
}

func getPieChartData(g *gin.Context) {

	//返却用構造体
	var varchart []VarChart
	//scan用変数
	var name, value string

	// コネクション作成
	db, err := sql.Open("sqlserver", getconnString("ConnectionString.txt"))
	if err != nil {
		panic(err.Error())
	}
	// 必ずclose＠おまじない
	defer db.Close()

	//SQL実行
	rows, err := db.Query(getSQL("S-getPieChartData.txt"))

	if err != nil {
		panic(err.Error())
	}

	//取得値をscanして構造体に格納
	for rows.Next() {
		strange := rows.Scan(&name, &value)
		if strange != nil {
			log.Fatal(strange)
		}
		varchart = append(varchart, VarChart{Name: name, Value: value})
	}

	//JSONにして返却
	g.JSON(200, varchart)

	return
}

func getListData(g *gin.Context) {

	//SQLパラメータ用変数の定義
	var param Param
	
	//SQLパラメータ用変数の変数にバインド
	g.BindJSON(&param)

	//返却用構造体
	var returnTest []ReturnTest

	//パラメーター別SQLの部分
	var ParamSql string

	//実行SQL
	sqlStringHeader := "SELECT * FROM ( SELECT employee_id id, employee_nm name, "
	sqlStringFooter := " overtime FROM kintai_imp WHERE pay_target_date IN( select pay_target_date FROM ( SELECT distinct TOP (@top) pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC) tmp ) GROUP BY employee_id,employee_nm ) target WHERE overtime >= @overtime ORDER BY overtime desc"

	//条件別SQL
	allOver := "(sum(kinmu_time) - sum(shotei_kinmu_count))"
	houteigai := "sum(houteigai_zangyo_time)"
	sinya := "sum(sinya_kinmu_time)"
	houteinai := "sum(houteinai_zangyo_time)"

	allOverAverage := "(sum(kinmu_time) - sum(shotei_kinmu_count))/"
	houteigaiAverage := "sum(houteigai_zangyo_time)/"
	sinyaAverage := "sum(sinya_kinmu_time)/"
	houteinaiAverage := "sum(houteinai_zangyo_time)/"

	//limit数
	sqlLimit := 3

	//パラメータに応じてSQLを選択
	switch param.SqlTarget + param.SqlTerm {
		//総残業直近1か月合計
		case "00":
			sqlLimit = 1
			ParamSql = allOver
		//総残業直近3か月合計
		case "01":
			ParamSql = allOver
		//総残業直近3か月平均
		case "02":
			ParamSql = allOverAverage + "3"
		//普通残業直近1か月合計
		case "10":
			sqlLimit = 1
			ParamSql = houteigai
		//普通残業直近3か月合計
		case "11":
			ParamSql = houteigai
		//普通残業直近3か月平均
		case "12":
			ParamSql = houteigaiAverage + "3"
		//深夜残業直近1か月合計
		case "20":
			sqlLimit = 1
			ParamSql = sinya
		//深夜残業直近3か月合計
		case "21":
			ParamSql = sinya
		//深夜残業直近3か月平均
		case "22":
			ParamSql = sinyaAverage  + "3"
		//深夜残業直近1か月合計
		case "30":
			sqlLimit = 1
			ParamSql = houteinai
		//深夜残業直近3か月合計
		case "31":
			ParamSql = houteinai
		//深夜残業直近3か月平均
		case "32":
			ParamSql = houteinaiAverage + "3"
	}

	//scan用変数
	var Id, Name, OverTime string

	// コネクション作成
	db, err := sql.Open("sqlserver", getconnString("ConnectionString.txt"))
	if err != nil {
		panic(err.Error())
	}
	// 必ずclose＠おまじない
	defer db.Close()

	//SQL実行
	rows, err := db.Query(sqlStringHeader + ParamSql + sqlStringFooter, 
		sql.NamedArg{ Name: "top", Value: sqlLimit },
		sql.NamedArg{ Name: "overtime", Value: param.SqlTime },
	)

	if err != nil {
		panic(err.Error())
	}

	//取得値をscanして構造体に格納
	for rows.Next() {
		strange := rows.Scan(&Id, &Name, &OverTime)
		if strange != nil {
			log.Fatal(strange)
		}
		returnTest = append(returnTest, ReturnTest{Id: Id, Name: Name, OverTime: OverTime})
	}

	//JSONにして返却
	g.JSON(200, returnTest)

	return
}

func Logout(g *gin.Context) {

    //セッションからデータを破棄する
    session := sessions.Default(g)
    //log.Println("セッション取得")
    session.Clear()
    //log.Println("クリア処理")
    session.Save()

}

func getSQL(filename string) string {
    text, err := ioutil.ReadFile("./SQL/" + filename)
    if err != nil {
        fmt.Println(err)
    }
    return string(text)
}

func getconnString(filename string) string {
    text, err := ioutil.ReadFile("./" + filename)
    if err != nil {
        fmt.Println(err)
    }
    return string(text)
}