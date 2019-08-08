package main

import (
	"fmt"
	"mime/multipart"
	"os"
	"strings"
	"time"
	"strconv"
	//"path/filepath"

	"github.com/kataras/iris"
	"github.com/kataras/iris/middleware/logger"
	"github.com/kataras/iris/middleware/recover"
	//"github.com/kataras/iris/mvc"
	"github.com/kataras/iris/websocket"
	"github.com/kataras/iris/sessions"

	"gopkg.in/go-playground/validator.v9"
	"github.com/go-redis/redis"

	"myiris/library"
)

func main() {
	//current_path, _ := filepath.Abs(`.`)
	var GOPATH = os.Getenv("GOPATH")
	arg_list := os.Args
	var config_dir string
	var PROJECT_DIR = GOPATH + "/src/myiris/"
	if len(arg_list) >1 {
		config_dir = arg_list[1]
	}else {
		config_dir = PROJECT_DIR + "config"
	}
	if library.Substr(config_dir,-1,1) != "/" {
		config_dir = config_dir + "/"
	}
	if !library.PathExists(config_dir) {
		fmt.Println(config_dir)
		os.Exit(1)
	}

	app := iris.New()
	// Recover middleware recovers from any panics and writes a 500 if there was one.
	app.Use(recover.New())

	requestLogger := logger.New(logger.Config{
		Status: true,
		IP: true,
		Method: true,
		Path: true,
		Query: true,
		// if !empty then its contents derives from `ctx.Values().Get("logger_message")
		// will be added to the logs.
		MessageContextKeys: []string{"logger_message"},
		// if !empty then its contents derives from `ctx.GetHeader("User-Agent")
		MessageHeaderKeys: []string{"User-Agent"},
	})
	app.Use(requestLogger)

	f := newLogFile()
	defer f.Close()
	// Attach the file as logger, remember, iris' app logger is just an io.Writer.
	// Use the following code if you need to write the logs to file and console at the same time.
	// app.Logger().SetOutput(io.MultiWriter(f, os.Stdout))

	app.Logger().SetOutput(f)

	//错误处理handle 404 500
	app.OnErrorCode(iris.StatusNotFound, notFound)
	app.OnErrorCode(iris.StatusInternalServerError, internalServerError)

	// 每一个请求都会先执行此方法 app.Use()
	// 注册 "before"  处理器作为当前域名所有路由中第一个处理函数
	// 或者使用  `UseGlobal`  去注册一个中间件，用于在所有子域名中使用
	app.Use(before)
	// 注册  "after" ，在所有路由的处理程序之后调用
	app.Done(after)

	// Method:   GET
	// Resource: http://localhost:8080
	app.Handle("GET", "/", func(ctx iris.Context) {
		ctx.HTML("<h1>Welcome</h1>")
	})


	// same as app.Handle("GET", "/ping", [...])
	// Method:   GET
	// Resource: http://localhost:8080/ping
	app.Get("/ping", func(ctx iris.Context) {
		type MyUser struct {
			Id int `json:"id"`
			Name string `json:"name"`
			Email string `json:"email"`
		}
		user := MyUser{
			Id:1,
			Name:"levsion",
			Email:"levsion@163.com",
		}
		ctx.JSON(user)
		ctx.XML(user)
		ctx.WriteString(time.Now().Format("2006-01-02 15:04:05") + " pong")
	})

	// Method:   GET
	// Resource: http://localhost:8080/hello
	app.Get("/hello/{name:string}", func(ctx iris.Context) {
		name := ctx.Params().Get("name")
		ctx.JSON(iris.Map{"message": "Hello " + name})
		app.Logger().Infof("welcome %s, we are family !!!",  name)
	})

	app.Post("/post", func(ctx iris.Context) {
		id := ctx.URLParam("id")
		page := ctx.URLParamDefault("page", "0")
		name := ctx.FormValue("name")
		message := ctx.FormValue("message")
		// or `ctx.PostValue` for POST, PUT & PATCH-only HTTP Methods.
		app.Logger().Infof("id: %s; page: %s; name: %s; message: %s", id, page, name, message)
	})

	app.Post("/upload", iris.LimitRequestBodySize(5<<20), func(ctx iris.Context) {
		//
		// UploadFormFiles
		// uploads any number of incoming files ("multiple" property on the form input).
		//

		// The second, optional, argument
		// can be used to change a file's name based on the request,
		// at this example we will showcase how to use it
		// by prefixing the uploaded file with the current user's ip.
		ctx.UploadFormFiles("./static/images", beforeSave)
	})

	group1 := app.Party("/group",TheMiddleware)
	{
		group1.Get("/login", func(ctx iris.Context) {
			ctx.WriteString("I am login");
		})
		group1.Get("/submit", func(ctx iris.Context) {
			ctx.WriteString("I am submit");
		})
		group1.Get("/read", func(ctx iris.Context) {
			ctx.WriteString("I am read");
		})
	}

	//validator struct
	type Address struct {
		Street string `json:"street" validate:"required"`
		City   string `json:"city" validate:"required"`
		Planet string `json:"planet" validate:"required"`
		Phone  string `json:"phone" validate:"required"`
	}
	type User struct {
		FirstName      string     `json:"fname"`
		LastName       string     `json:"lname"`
		Age            uint8      `json:"age" validate:"gte=0,lte=130"`
		Email          string     `json:"email" validate:"required,email"`
		FavouriteColor string     `json:"favColor" validate:"hexcolor|rgb|rgba"`
		Addresses      []*Address `json:"addresses" validate:"required,dive,required"`
	}
	validate := validator.New()
	app.Post("/adduser", func(ctx iris.Context) {
		var user User
		if err := ctx.ReadJSON(&user); err != nil {
			// Handle error.
		}

		// Returns InvalidValidationError for bad validation input,
		// nil or ValidationErrors ( []FieldError )
		err := validate.Struct(user)
		if err != nil {

			// This check is only needed when your code could produce
			// an invalid value for validation such as interface with nil
			// value most including myself do not usually have code like this.
			if _, ok := err.(*validator.InvalidValidationError); ok {
				ctx.StatusCode(iris.StatusInternalServerError)
				ctx.WriteString(err.Error())
				return
			}

			ctx.StatusCode(iris.StatusBadRequest)
			for _, err := range err.(validator.ValidationErrors) {
				fmt.Println()
				fmt.Println(err.Namespace())
				fmt.Println(err.Field())
				fmt.Println(err.StructNamespace())
				fmt.Println(err.StructField())
				fmt.Println(err.Tag())
				fmt.Println(err.ActualTag())
				fmt.Println(err.Kind())
				fmt.Println(err.Type())
				fmt.Println(err.Value())
				fmt.Println(err.Param())
				fmt.Println()
			}
			return
		}
		// save user to database
	})


	app.Get("/websocket", func(ctx iris.Context) {
		ctx.ServeFile("views/websockets.html", false) // second parameter: enable gzip?
	})
	setupWebsocket(app)

	// Set A Cookie.
	app.Get("/cookies/{name}/{value}", func(ctx iris.Context) {
		name := ctx.Params().Get("name")
		value := ctx.Params().Get("value")

		ctx.SetCookieKV(name, value)
		//value := ctx.GetCookie(name)
		//ctx.RemoveCookie(name)

		ctx.Writef("cookie added: %s = %s", name, value)
	})

	// Set A Session
	var cookieNameForSessionID = "mycookiesessionnameid"
	sess:= sessions.New(sessions.Config{Cookie: cookieNameForSessionID})
	app.Get("/sessions",func(ctx iris.Context) {
		session := sess.Start(ctx)
		session.Set("authenticated", "hello session")
		auth_str := session.GetString("authenticated")
		ctx.WriteString(auth_str)
		session.Delete("authenticated")
	})

	//　从　"./views"　目录下加载扩展名是".html" 　的所有模板，
	//　并使用标准的　`html/template`　 包进行解析。
	app.RegisterView(iris.HTML(PROJECT_DIR+"views/", ".html"))
	app.Get("/views", func(ctx iris.Context) {
		// 绑定： {{.message}}　为　"Hello world!"
		ctx.ViewData("message", "Hello world!")
		ctx.ViewData("title", "This is a view html")
		// 渲染模板文件： ./views/hello.html
		ctx.View("hello.html")
	})

	client := redis.NewClient(&redis.Options{
		Addr:     "127.0.0.1:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})
	app.Get("/redis",func(ctx iris.Context){
		_, err := client.Ping().Result()
		if err != nil {
			panic(err)
		}
		key := "key"
		err_ := client.Set(key, "value", 0).Err()
		if err_ != nil {
			panic(err_)
		}

		val, err__ := client.Get("key").Result()
		if err__ != nil {
			panic(err__)
		}
		ctx.Writef("redis success: %s = %s", key, val)
	})

	// http://localhost:8080
	// http://localhost:8080/ping
	// http://localhost:8080/hello
	_ = app.Run(iris.Addr(":8090"),iris.WithConfiguration(iris.TOML(config_dir+"main.tml")))
}

func TheMiddleware(ctx iris.Context) {
	//if auth_str := session.GetString("authenticated"); !auth_str {
	//	return
	//}

	//ctx.Next 继续往下一个处理方法 中间件 如果没有他 那么就不会执行 usersRoutes
	ctx.Next()
}

func setupWebsocket(app *iris.Application) {
	// create our echo websocket server
	ws := websocket.New(websocket.Config{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	})
	ws.OnConnection(handleConnection)

	// register the server on an endpoint.
	// see the inline javascript code in the websockets.html,
	// this endpoint is used to connect to the server.
	app.Get("/echo", ws.Handler())
	// serve the javascript built-in client-side library,
	// see websockets.html script tags, this path is used.
	app.Any("/iris-ws.js", websocket.ClientHandler())
}

func handleConnection(c websocket.Connection) {
	// Read events from browser
	c.On("chat", func(msg string) {
		// Print the message to the console, c.Context() is the iris's http context.
		fmt.Printf("%s sent: %s\n", c.Context().RemoteAddr(), msg)
		// Write message back to the client message owner:
		//c.Emit("chat", msg)
		// Write message to all except this client with:
		c.To(websocket.Broadcast).Emit("chat", c.Context().RemoteAddr()+ ": " + msg)
	})
}

func beforeSave(ctx iris.Context, file *multipart.FileHeader) {
	ip := ctx.RemoteAddr()
	// make sure you format the ip in a way
	// that can be used for a file name (simple case):
	ip = strings.Replace(ip, ".", "_", -1)
	ip = strings.Replace(ip, ":", "_", -1)

	// you can use the time.Now, to prefix or suffix the files
	// based on the current time as well, as an exercise.
	// i.e unixTime :=	time.Now().Unix()
	// prefix the Filename with the $IP-
	// no need for more actions, internal uploader will use this
	// name to save the file into the "./uploads" folder.
	file.Filename = ip + "-" + file.Filename
	file.Filename = time.Now().Format("2006-01-02 15:04:05") + "-" + file.Filename
}

func newLogFile() *os.File {
	log_path := "/Users/levsion/Documents/code/go/src/myiris/"
	filename := log_path + "log/"+strconv.Itoa(time.Now().Year()) + time.Now().Month().String() + strconv.Itoa(time.Now().Day()) +".log"
	// Open the file, this will append to the today's file if server restarted.
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		panic(err)
	}

	return f
}

func before(ctx iris.Context) {
	//
	//fmt.Println("before request ")
	ctx.Next()
}
func after(ctx iris.Context) {
	fmt.Println("after request ")
	ctx.Next()
}

func notFound(ctx iris.Context) {
	// 出现 404 的时候，就跳转到 $views_dir/errors/404.html 模板
	//ctx.View("errors/404.html")
	ctx.WriteString("The page not found !!!")
}

func internalServerError(ctx iris.Context) {
	ctx.WriteString("Oups something went wrong, try again")
}