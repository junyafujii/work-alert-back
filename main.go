package main

import (
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
)

type User struct {
	UserId   string `json:"userid"`
	Password string `json:"password"`
}
type Cnt struct {
	Count string `json:"usercnt"`
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

var ConnectionString string = "root:p@ssw0rd@/study"
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
	server.GET("/sessionCheck", sessionCheck)

	server.POST("/signin", signin)
	server.POST("/getListData", getListData)
	server.POST("/impcsv", impcsv)

	//指定のポート番号でサーバー実行
	server.Run(":8888")
}

func sessionCheck(g *gin.Context) {
	session := sessions.Default(g)
	Info.UserId = session.Get("UserId")

	// セッションがない場合
	if Info.UserId == nil {
		// g.JSON(440,"セッションが切れています。ログインしなおしてください。")
	// セッションがある場合
	} else {
		//何もしない？
	}
}

func signin(g *gin.Context) {

	//構造体定義
	var user User
	var cnt Cnt

	// 第2引数の形式は "user:password@tcp(host:port)/dbname"
	db, err := sql.Open("mysql", ConnectionString)
	if err != nil {
		panic(err.Error())
	}
	defer db.Close()

	//ログインユーザー用変数の変数にバインド
	g.BindJSON(&user)

	// Select文発行
	err = db.QueryRow(
      "SELECT " + 
           "cast(count(id) as char) usercnt " +
      "FROM " +
		  "study.login_user " +
	  "WHERE " + 
	  	 "id = '" + user.UserId + "' " + 
		 "and password = '" + user.Password + "'").Scan(&(cnt.Count))

	// エラーがない場合、セッションセット
	if err == nil {
		session := sessions.Default(g)
		session.Options(sessions.Options{MaxAge: 600})
		session.Set("UserId", user.UserId)
		session.Save()
		//JSONにして返却
		g.JSON(200, gin.H{"UserID": user.UserId, "usercnt":cnt.Count})
	// エラーの場合
	}else{
		panic(err.Error())
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
		if len(record[value]) != 85 {
			return 2
		}
	}

	//何も問題なければ0
	return 0
}

func execInsert(record [][]string) {

	// insert文雛形
	sqlbase := "INSERT INTO study.kintai_imp VALUES (?,?,str_to_date(?,'%Y/%m/%d'),?,?,?,?,?,?,?,?,?,str_to_date(?,'%Y/%m/%d'),str_to_date(?,'%Y/%m/%d'),?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)"

	// コネクション作成
	db, err := sql.Open("mysql", "root:p@ssw0rd@/study")
	if err != nil {
		panic(err.Error())
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
				sqlbase,
				record[i][0],
				record[i][1],
				record[i][2],
				record[i][3],
				record[i][4],
				record[i][5],
				record[i][6],
				record[i][7],
				record[i][8],
				record[i][9],
				record[i][10],
				record[i][11],
				record[i][12],
				record[i][13],
				record[i][14],
				record[i][15],
				record[i][16],
				record[i][17],
				record[i][18],
				record[i][19],
				record[i][20],
				record[i][21],
				record[i][22],
				record[i][23],
				record[i][24],
				record[i][25],
				record[i][26],
				record[i][27],
				record[i][28],
				record[i][29],
				record[i][30],
				record[i][31],
				record[i][32],
				record[i][33],
				record[i][34],
				record[i][35],
				record[i][36],
				record[i][37],
				record[i][38],
				record[i][39],
				record[i][40],
				record[i][41],
				record[i][42],
				record[i][43],
				record[i][44],
				record[i][45],
				record[i][46],
				record[i][47],
				record[i][48],
				record[i][49],
				record[i][50],
				record[i][51],
				record[i][52],
				record[i][53],
				record[i][54],
				record[i][55],
				record[i][56],
				record[i][57],
				record[i][58],
				record[i][59],
				record[i][60],
				record[i][61],
				record[i][62],
				record[i][63],
				record[i][64],
				record[i][65],
				record[i][66],
				record[i][67],
				record[i][68],
				record[i][69],
				record[i][70],
				record[i][71],
				record[i][72],
				record[i][73],
				record[i][74],
				record[i][75],
				record[i][76],
				record[i][77],
				record[i][78],
				record[i][79],
				record[i][80],
				record[i][81],
				record[i][82],
				record[i][83],
				record[i][84])

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
	db, err := sql.Open("mysql", "root:p@ssw0rd@/study")
	if err != nil {
		panic(err.Error())
	}
	// 必ずclose＠おまじない
	defer db.Close()

	//SQL実行
	rows, err := db.Query(
		"SELECT "+
			"'80時間以上' NAME "+
			",COUNT(employee_id) cnt "+
		"FROM "+
		"( "+
			"SELECT "+
				"employee_id "+
				",(sum(kinmu_time) - sum(shotei_kinmu_count))/3 heikinzangyou "+
			"FROM "+
				"kintai_imp "+
			"WHERE "+
				"pay_target_date IN (select pay_target_date FROM (SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT 3) tmp) "+
			"GROUP BY "+
				"employee_id "+
		") target "+
		"WHERE "+
			"heikinzangyou >= 80 "+
		"UNION ALL "+
			"SELECT "+
				"'60～80時間未満' NAME "+
				",COUNT(employee_id) cnt "+
		"FROM "+
		"( "+
			"SELECT "+
				"employee_id "+
				",(sum(kinmu_time) - sum(shotei_kinmu_count))/3 heikinzangyou "+
			"FROM "+
				"kintai_imp "+
			"WHERE "+
				"pay_target_date IN (select pay_target_date FROM (SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT 3) tmp) "+
			"GROUP BY "+
				"employee_id "+
		") target "+
		"WHERE "+
			"heikinzangyou BETWEEN 60 AND 79 "+
		"UNION ALL "+
		"SELECT "+
			"'45～60時間未満' NAME "+
			",COUNT(employee_id) cnt "+
		"FROM "+
		"( "+
			"SELECT "+
				"employee_id "+
				",(sum(kinmu_time) - sum(shotei_kinmu_count))/3 heikinzangyou "+
			"FROM "+
				"kintai_imp "+
			"WHERE "+
				"pay_target_date IN (select pay_target_date FROM (SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT 3) tmp) "+
			"GROUP BY "+
				"employee_id "+
		") target "+
		"WHERE "+
			"heikinzangyou BETWEEN 45 AND 59")
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
	db, err := sql.Open("mysql", "root:p@ssw0rd@/study")
	if err != nil {
		panic(err.Error())
	}
	// 必ずclose＠おまじない
	defer db.Close()

	//SQL実行
	rows, err := db.Query(
		"SELECT "+
			"'80時間以上' NAME "+
			",COUNT(employee_id) cnt "+
		"FROM "+
		"( "+
			"SELECT "+
				"employee_id "+
				",(sum(kinmu_time) - sum(shotei_kinmu_count))/3 heikinzangyou "+
			"FROM "+
				"kintai_imp "+
			"WHERE "+
				"pay_target_date IN (select pay_target_date FROM (SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT 3) tmp) "+
			"GROUP BY "+
				"employee_id "+
		") target "+
		"WHERE "+
			"heikinzangyou >= 80 "+
		"UNION ALL "+
			"SELECT "+
				"'60～80時間未満' NAME "+
				",COUNT(employee_id) cnt "+
		"FROM "+
		"( "+
			"SELECT "+
				"employee_id "+
				",(sum(kinmu_time) - sum(shotei_kinmu_count))/3 heikinzangyou "+
			"FROM "+
				"kintai_imp "+
			"WHERE "+
				"pay_target_date IN (select pay_target_date FROM (SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT 3) tmp) "+
			"GROUP BY "+
				"employee_id "+
		") target "+
		"WHERE "+
			"heikinzangyou BETWEEN 60 AND 79 "+
		"UNION ALL "+
		"SELECT "+
			"'45～60時間未満' NAME "+
			",COUNT(employee_id) cnt "+
		"FROM "+
		"( "+
			"SELECT "+
				"employee_id "+
				",(sum(kinmu_time) - sum(shotei_kinmu_count))/3 heikinzangyou "+
			"FROM "+
				"kintai_imp "+
			"WHERE "+
				"pay_target_date IN (select pay_target_date FROM (SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT 3) tmp) "+
			"GROUP BY "+
				"employee_id "+
		") target "+
		"WHERE "+
			"heikinzangyou BETWEEN 45 AND 59")

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
	sqlStringFooter := " overtime FROM kintai_imp WHERE pay_target_date IN( select pay_target_date FROM ( SELECT distinct pay_target_date FROM kintai_imp ORDER BY pay_target_date DESC LIMIT ? ) tmp ) GROUP BY employee_id ) target WHERE overtime >= ? ORDER BY overtime desc"

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
	sqlLimit := "3"

	//パラメータに応じてSQLを選択
	switch param.SqlTarget + param.SqlTerm {
		//総残業直近1か月合計
		case "00":
			sqlLimit = "1"
			ParamSql = allOver
		//総残業直近3か月合計
		case "01":
			ParamSql = allOver
		//総残業直近3か月平均
		case "02":
			ParamSql = allOverAverage + sqlLimit
		//普通残業直近1か月合計
		case "10":
			sqlLimit = "1"
			ParamSql = houteigai
		//普通残業直近3か月合計
		case "11":
			ParamSql = houteigai
		//普通残業直近3か月平均
		case "12":
			ParamSql = houteigaiAverage + sqlLimit
		//深夜残業直近1か月合計
		case "20":
			sqlLimit = "1"
			ParamSql = sinya
		//深夜残業直近3か月合計
		case "21":
			ParamSql = sinya
		//深夜残業直近3か月平均
		case "22":
			ParamSql = sinyaAverage  + sqlLimit
		//深夜残業直近1か月合計
		case "30":
			sqlLimit = "1"
			ParamSql = houteinai
		//深夜残業直近3か月合計
		case "31":
			ParamSql = houteinai
		//深夜残業直近3か月平均
		case "32":
			ParamSql = houteinaiAverage + sqlLimit
	}

	//scan用変数
	var Id, Name, OverTime string

	// コネクション作成
	db, err := sql.Open("mysql", "root:p@ssw0rd@/study")
	if err != nil {
		panic(err.Error())
	}
	// 必ずclose＠おまじない
	defer db.Close()

	//SQL実行
	rows, err := db.Query(sqlStringHeader + ParamSql + sqlStringFooter, sqlLimit, param.SqlTime)
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
