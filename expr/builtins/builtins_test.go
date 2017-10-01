package builtins

import (
	"encoding/json"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/araddon/dateparse"
	u "github.com/araddon/gou"
	"github.com/stretchr/testify/assert"

	"github.com/araddon/qlbridge/datasource"
	"github.com/araddon/qlbridge/expr"
	"github.com/araddon/qlbridge/value"
	"github.com/araddon/qlbridge/vm"
)

var _ = u.EMPTY

func init() {
	u.SetupLogging("debug")
	u.SetColorOutput()
	LoadAllBuiltins()
}

type testBuiltins struct {
	expr string
	val  value.Value
}

// Our test struct, try as many different field types as possible
type User struct {
	Name          string
	Created       time.Time
	Updated       *time.Time
	Authenticated bool
	HasSession    *bool
	Roles         []string
	BankAmount    float64
	Address       Address
	Data          json.RawMessage
	Context       u.JsonHelper
	Hits          map[string]int64
	FirstEvent    map[string]time.Time
}
type Address struct {
	City string
	Zip  int
}

func (m *User) FullName() string {
	return m.Name + ", Jedi"
}

var (
	// This is used so we have a constant understood time for message context
	// normally we would use time.Now()
	//   "Apr 7, 2014 4:58:55 PM"

	regDate     = "10/13/2014"
	ts          = time.Date(2014, 4, 7, 16, 58, 55, 00, time.UTC)
	ts2         = time.Date(2014, 4, 7, 0, 0, 0, 00, time.UTC)
	regTime     = dateparse.MustParse(regDate)
	pst, _      = time.LoadLocation("America/Los_Angeles")
	readContext = datasource.NewContextUrlValuesTs(url.Values{
		"event":        {"hello"},
		"reg_date":     {"10/13/2014"},
		"msdate":       {"1438445529707"},
		"price":        {"$55"},
		"email":        {"email@email.com"},
		"emails":       {"email1@email.com", "email2@email.com"},
		"url":          {"http://www.site.com/membership/all.html"},
		"score_amount": {"22"},
		"tag_name":     {"bob"},
		"tags":         {"a", "b", "c", "d"},
		"sval":         {"event43,event4=63.00,event228"},
		"ua":           {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.11 (KHTML, like Gecko) Chrome/23.0.1271.97 Safari/537.11"},
		"json":         {`[{"name":"n1","ct":8,"b":true, "tags":["a","b"]},{"name":"n2","ct":10,"b": false, "tags":["a","b"]}]`},
	}, ts)
	float3pt1 = float64(3.1)
)

var builtinTestsx = []testBuiltins{
	{`json.jmespath(json, "[?name == 'n1'].name | [0]")`, value.NewStringValue("n1")},
}
var builtinTests = []testBuiltins{

	/*
		Logical bool Evaluation Functions
		Evaluate to true/false
	*/
	{`eq(5,5)`, value.BoolValueTrue},
	{`eq("hello", event)`, value.BoolValueTrue},
	{`eq(5,6)`, value.BoolValueFalse},
	{`eq(5.5,6)`, value.BoolValueFalse},
	{`eq(true,eq(5,5))`, value.BoolValueTrue},
	{`eq(true,false)`, value.BoolValueFalse},
	{`eq(not_a_field,5)`, value.BoolValueFalse},
	{`eq(eq(not_a_field,5),false)`, value.BoolValueTrue},

	{`ne(5,5)`, value.BoolValueFalse},
	{`ne("hello", event)`, value.BoolValueFalse},
	{`ne("hello", fakeevent)`, value.BoolValueTrue},
	{`ne(5,6)`, value.BoolValueTrue},
	{`ne(true,eq(5,5))`, value.BoolValueFalse},
	{`ne(true,false)`, value.BoolValueTrue},
	{`ne(oneof(event,"yes"),"")`, value.BoolValueTrue},
	{`eq(oneof(fakeevent,"yes"),"yes")`, value.BoolValueTrue},

	{`not(true)`, value.BoolValueFalse},
	{`not(eq(5,6))`, value.BoolValueTrue},
	{`not(eq(5,not_a_field))`, value.BoolValueTrue},
	{`not(eq(5,len("12345")))`, value.BoolValueFalse},
	{`not(eq(5,len(not_a_field)))`, value.BoolValueTrue},

	{`ge(5,5)`, value.BoolValueTrue},
	{`ge(5,6)`, value.BoolValueFalse},
	{`ge(5,3)`, value.BoolValueTrue},
	{`ge(5.5,3)`, value.BoolValueTrue},
	{`ge(5,"3")`, value.BoolValueTrue},

	{`le(5,5)`, value.BoolValueTrue},
	{`le(5,6)`, value.BoolValueTrue},
	{`le(5,3)`, value.BoolValueFalse},
	{`le(5,"3")`, value.BoolValueFalse},

	{`lt(5,5)`, value.BoolValueFalse},
	{`lt(5,6)`, value.BoolValueTrue},
	{`lt(5,3)`, value.BoolValueFalse},
	{`lt(5,"3")`, value.BoolValueFalse},

	{`gt(5,5)`, value.BoolValueFalse},
	{`gt(5,6)`, value.BoolValueFalse},
	{`gt(5,3)`, value.BoolValueTrue},
	{`gt(5,"3")`, value.BoolValueTrue},
	{`gt(5,toint("3.5"))`, value.BoolValueTrue},
	{`gt(toint(total_amount),0)`, nil}, // error because no total_amount?
	{`gt(toint(total_amount),0) || true`, value.BoolValueTrue},
	{`gt(toint(price),1)`, value.BoolValueTrue},

	{`exists(event)`, value.BoolValueTrue},
	{`exists(price)`, value.BoolValueTrue},
	{`exists(toint(price))`, value.BoolValueTrue},
	{`exists(-1)`, value.BoolValueTrue},
	{`exists(non_field)`, value.BoolValueFalse},

	/*
		Logical Bool evaluation of List/Array types
	*/

	{`any(5)`, value.BoolValueTrue},
	{`any("value")`, value.BoolValueTrue},
	{`any(event)`, value.BoolValueTrue},
	{`any(notrealfield)`, value.BoolValueFalse},

	{`all("Apple")`, value.BoolValueTrue},
	{`all("Apple")`, value.BoolValueTrue},
	{`all("Apple",event)`, value.BoolValueTrue},
	{`all("Apple",event,true)`, value.BoolValueTrue},
	{`all("Apple",event)`, value.BoolValueTrue},
	{`all("Linux",true,not_a_realfield)`, value.BoolValueFalse},
	{`all("Linux",false)`, value.BoolValueFalse},
	{`all("Linux","")`, value.BoolValueFalse},
	{`all("Linux",notreal)`, value.BoolValueFalse},

	{`oneof("apples","oranges")`, value.NewStringValue("apples")},
	{`oneof(notincontext,event)`, value.NewStringValue("hello")},
	{`oneof(not_a_field, email("Bob <bob@bob.com>"))`, value.NewStringValue("bob@bob.com")},
	{`oneof(email, email(not_a_field))`, value.NewStringValue("email@email.com")},
	{`oneof(email, email(not_a_field)) NOT IN ("a","b",10, 4.5) `, value.NewBoolValue(true)},
	{`oneof(email, email(not_a_field)) IN ("email@email.com","b",10, 4.5) `, value.NewBoolValue(true)},
	{`oneof(email, email(not_a_field)) IN ("b",10, 4.5) `, value.NewBoolValue(false)},

	/*
		Map, List, Array functions
	*/
	{`map(event, 22)`, value.NewMapValue(map[string]interface{}{"hello": 22})},
	{`map(event, toint(score_amount))`, value.NewMapValue(map[string]interface{}{"hello": 22})},

	{`maptime(event)`, value.NewMapTimeValue(map[string]time.Time{"hello": ts})},
	{`maptime(event, "2016-02-03T22:00:00")`, value.NewMapTimeValue(map[string]time.Time{"hello": time.Date(2016, 2, 3, 22, 0, 0, 0, time.UTC)})},

	{`filtermatch(split(sval,","),"event4=")`, value.NewStringsValue([]string{"event4=63.00"})},
	{`filtermatch(match("score_","tag_"),"amo*")`, value.NewMapValue(map[string]interface{}{"amount": "22"})},

	{`filter(match("score_","tag_"),"nam*")`, value.NewMapValue(map[string]interface{}{"amount": "22"})},
	{`filter(match("score_","tag_"),"name")`, value.NewMapValue(map[string]interface{}{"amount": "22"})},
	{`filter(split("apples,oranges",","),"ora*")`, value.NewStringsValue([]string{"apples"})},
	{`filter(split("apples,oranges",","), ["ora*","notmatch","stuff"] )`, value.NewStringsValue([]string{"apples"})},

	{`match("score_")`, value.NewMapValue(map[string]interface{}{"amount": "22"})},
	{`match("score_","tag_")`, value.NewMapValue(map[string]interface{}{"amount": "22", "name": "bob"})},
	{`mapkeys(match("score_","tag_"))`, value.NewStringsValue([]string{"amount", "name"})},
	{`mapvalues(match("score_","tag_"))`, value.NewStringsValue([]string{"22", "bob"})},
	{`mapvalues(will_not_match)`, value.NewStringsValue(nil)},
	{`mapinvert(match("score_","tag_"))`, value.NewMapStringValue(map[string]string{"22": "amount", "bob": "name"})},
	{`match("nonfield_")`, value.ErrValue},

	{`len(["5","6"])`, value.NewIntValue(2)},
	{`len(split(reg_date,"/"))`, value.NewIntValue(3)},

	// "tags":         {"a", "b", "c", "d"},
	{`array.index(tags,1)`, value.NewStringValue("b")},
	{`array.index(tags, -1)`, value.NewStringValue("d")},
	{`array.index(tags,-2)`, value.NewStringValue("c")},
	{`array.index(tags,6)`, nil},
	{`array.index(tags,-6)`, nil},
	{`array.slice(tags,2)`, value.NewStringsValue([]string{"c", "d"})},
	{`array.slice(tags,-2)`, value.NewStringsValue([]string{"c", "d"})},
	{`array.slice(tags,-1)`, value.NewStringsValue([]string{"d"})},
	{`array.slice(tags,1,3)`, value.NewStringsValue([]string{"b", "c"})},
	{`array.slice(tags,1,4)`, value.NewStringsValue([]string{"b", "c", "d"})},
	{`array.slice(tags,-3,-1)`, value.NewStringsValue([]string{"b", "c"})},
	{`array.slice(tags,1,7)`, value.ErrValue},
	{`array.slice(tags,1,-7)`, value.ErrValue},
	{`array.slice(tags,-1,77)`, value.ErrValue},

	/*
		String Functions
	*/

	{`contains("5tem",5)`, value.BoolValueTrue},
	{`contains("5item","item")`, value.BoolValueTrue},
	{`contains("the-hello",event)`, value.BoolValueTrue},
	{`contains("the-item",event)`, value.BoolValueFalse},
	{`contains(price,"$")`, value.BoolValueTrue},
	{`contains(url,"membership/all.html")`, value.BoolValueTrue},
	{`contains(not_a_field,"nope")`, value.BoolValueFalse},
	{`false == contains(not_a_field,"nope")`, value.BoolValueTrue},
	{`contains(tags, Address)`, value.BoolValueFalse},
	{`contains(tags, "")`, value.ErrValue},

	{`hasprefix("5tem",5)`, value.BoolValueTrue},
	{`hasprefix("hello world",event)`, value.BoolValueTrue},
	{`hasprefix(event,"he")`, value.BoolValueTrue},
	{`hasprefix(event,"ham")`, value.BoolValueFalse},
	{`hasprefix("5tem","5y")`, value.BoolValueFalse},
	{`hasprefix("","5y")`, value.BoolValueFalse},
	{`hasprefix(not_a_field,"5y")`, value.BoolValueFalse},
	{`hasprefix("hello","")`, value.ErrValue},

	{`hassuffix("tem","m")`, value.BoolValueTrue},
	{`hassuffix("hello",event)`, value.BoolValueTrue},
	{`hassuffix(event,"lo")`, value.BoolValueTrue},
	{`hassuffix(event,"ham")`, value.BoolValueFalse},
	{`hassuffix("5tem","5y")`, value.BoolValueFalse},
	{`hassuffix("","5y")`, value.BoolValueFalse},
	{`hassuffix(not_a_field,"5y")`, value.BoolValueFalse},
	{`hassuffix("hello","")`, value.ErrValue},

	{`tolower("Apple")`, value.NewStringValue("apple")},
	{`tolower(Address)`, value.ErrValue},

	{`join("apple", event, "oranges", "--")`, value.NewStringValue("apple--hello--oranges")},
	{`join(["apple","peach"], ",")`, value.NewStringValue("apple,peach")},
	{`join("apple","","peach",",")`, value.NewStringValue("apple,peach")},
	{`join(split("apple,peach",","),"--")`, value.NewStringValue("apple--peach")},
	{`join("hello",Address)`, value.ErrValue},
	{`join(Address,"--")`, value.ErrValue},

	{`split("apples,oranges",",")`, value.NewStringsValue([]string{"apples", "oranges"})},
	{`split(Address,",")`, value.ErrValue},
	{`split("",",")`, value.ErrValue},
	{`split("hello","")`, value.ErrValue},

	{`strip("apples ")`, value.NewStringValue("apples")},
	{`strip(split("apples, oranges ",","))`, value.NewStringsValue([]string{"apples", "oranges"})},
	{`strip(split(" apples, oranges ",","))`, value.NewStringsValue([]string{"apples", "oranges"})},
	{`strip(split("apples
	, oranges ",","))`, value.NewStringsValue([]string{"apples", "oranges"})},
	{`strip(Address)`, value.ErrValue},

	{`replace("M20:30","M")`, value.NewStringValue("20:30")},
	{`replace("/search/for+stuff","/search/")`, value.NewStringValue("for+stuff")},
	{`replace("M20:30","M","")`, value.NewStringValue("20:30")},
	{`replace("M20:30","M","Hour ")`, value.NewStringValue("Hour 20:30")},

	// len is also a list operation above
	{`len("abc")`, value.NewIntValue(3)},
	{`len(not_a_field)`, nil},
	{`len(not_a_field) >= 10`, value.BoolValueFalse},
	{`len("abc") >= 2`, value.BoolValueTrue},
	{`CHAR_LENGTH("abc") `, value.NewIntValue(3)},
	{`CHAR_LENGTH(CAST("abc" AS CHAR))`, value.NewIntValue(3)},

	/*
		hashing functions
	*/
	{`hash.sip("http://www.google.com?q=123")`, value.NewIntValue(5673948842516703987)},
	{`hash.md5("hello")`, value.NewStringValue("5d41402abc4b2a76b9719d911017c592")},
	{`hash.sha1("hello")`, value.NewStringValue("aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d")},
	{`hash.sha256("hello")`, value.NewStringValue("2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824")},

	{`hash.sip("http://www.google.com?q=123") % 10`, value.NewIntValue(5673948842516703987 % 10)},

	{`encoding.b64encode("hello world")`, value.NewStringValue("aGVsbG8gd29ybGQ=")},
	{`encoding.b64decode("aGVsbG8gd29ybGQ=")`, value.NewStringValue("hello world")},

	/*
		Special Type Functions:  Email, url's
	*/

	{`email("Bob@Bob.com")`, value.NewStringValue("bob@bob.com")},
	{`email("Bob <bob>")`, value.ErrValue},
	{`email("Bob <bob@bob.com>")`, value.NewStringValue("bob@bob.com")},
	{`email(emails)`, value.NewStringValue("email1@email.com")},

	{`emailname("Bob<bob@bob.com>")`, value.NewStringValue("Bob")},
	{`emaildomain("Bob<bob@gmail.com>")`, value.NewStringValue("gmail.com")},

	{`host("https://www.Google.com/search?q=golang")`, value.NewStringValue("www.google.com")},
	{`host("www.Google.com/?q=golang")`, value.NewStringValue("www.google.com")},
	//{`host("notvalid")`, value.NewStringValue("notvalid")},
	{`hosts("www.Google.com/?q=golang", "www.golang.org/")`, value.NewStringsValue([]string{"www.google.com", "www.golang.org"})},

	{`urldecode("hello+world")`, value.NewStringValue("hello world")},
	{`urldecode("hello world")`, value.NewStringValue("hello world")},
	{`urldecode("2Live_Reg")`, value.NewStringValue("2Live_Reg")},
	{`urldecode("https%3A%2F%2Fwww.google.com%2Fsearch%3Fq%3Dgolang")`, value.NewStringValue("https://www.google.com/search?q=golang")},

	{`domain("https://www.Google.com/search?q=golang")`, value.NewStringValue("google.com")},
	{`domains("https://www.Google.com/search?q=golang")`, value.NewStringsValue([]string{"google.com"})},
	{`domains("https://www.Google.com/search?q=golang","http://www.ign.com")`, value.NewStringsValue([]string{"google.com", "ign.com"})},

	{`path("https://www.Google.com/search?q=golang")`, value.NewStringValue("/search")},
	{`path("https://www.Google.com/blog/hello.html")`, value.NewStringValue("/blog/hello.html")},
	{`path("www.Google.com/?q=golang")`, value.NewStringValue("/")},
	{`path("c://Windows/really")`, value.NewStringValue("//windows/really")},
	{`path("/home/aaron/vm")`, value.NewStringValue("/home/aaron/vm")},

	{`qs("https://www.Google.com/search?q=golang","q")`, value.NewStringValue("golang")},
	{`qs("www.Google.com/?q=golang","q")`, value.NewStringValue("golang")},

	{`url.matchqs("http://www.google.com/blog?mc_eid=123&mc_id=1&pid=123&utm_campaign=free")`, value.NewStringValue("www.google.com/blog")},
	{`url.matchqs("http://www.google.com/blog?mc_eid=123&mc_id=1&pid=123&utm_campaign=free", "pid", "mc_eid")`,
		value.NewStringValue("www.google.com/blog?mc_eid=123&pid=123")},
	{`url.matchqs("http://www.google.com/blog?mc_eid=123&mc_id=1&pid=123&utm_campaign=free", "mc_*")`,
		value.NewStringValue("www.google.com/blog?mc_eid=123&mc_id=1")},
	{`url.matchqs("http://www.google.com/blog")`, value.NewStringValue("www.google.com/blog")},
	{`url.matchqs("http://not a url")`, value.ErrValue},

	{`urlminusqs("http://www.Google.com/search?q1=golang&q2=github","q1")`, value.NewStringValue("http://www.Google.com/search?q2=github")},
	{`urlminusqs("http://www.Google.com/search?q1=golang&q2=github","q3")`, value.NewStringValue("http://www.Google.com/search?q1=golang&q2=github")},
	{`urlminusqs("http://www.Google.com/search?q1=golang","q1")`, value.NewStringValue("http://www.Google.com/search")},

	{`urlmain("http://www.Google.com/search?q1=golang&q2=github")`, value.NewStringValue("www.Google.com/search")},

	{`useragent(ua, "os")`, value.NewStringValue("Linux x86_64")},

	/*
		Casting and type-coercion functions
	*/
	{`cast(reg_date as time)`, value.NewTimeValue(regTime)},
	{`CAST(score_amount AS int))`, value.NewIntValue(22)},
	{`CAST(score_amount AS string))`, value.NewStringValue("22")},
	{`CAST(score_amount AS char))`, value.NewByteSliceValue([]byte("22"))},

	// ts2         = time.Date(2014, 4, 7, 0, 0, 0, 00, time.UTC)
	// Eu style
	{`todate("02/01/2006","07/04/2014")`, value.NewTimeValue(ts2)},
	{`todate("1/2/06","4/7/14")`, value.NewTimeValue(ts2)},
	{`todate("4/7/14")`, value.NewTimeValue(ts2)},
	{`todate("Apr 7, 2014 4:58:55 PM")`, value.NewTimeValue(ts)},
	{`todate("Apr 7, 2014 4:58:55 PM") < todate("now-3m")`, value.NewBoolValue(true)},

	{`todatein("May 8, 2009 5:57:51 PM","America/Los_Angeles")`, value.NewTimeValue(time.Date(2009, 5, 8, 17, 57, 51, 00, pst))},

	{`toint("5")`, value.NewIntValue(5)},
	{`toint("hello")`, value.ErrValue},
	{`toint("$ 5.22")`, value.NewIntValue(5)},
	{`toint("5.56")`, value.NewIntValue(5)},
	{`toint("$5.56")`, value.NewIntValue(5)},
	{`toint("5,555.00")`, value.NewIntValue(5555)},
	{`toint("€ 5,555.00")`, value.NewIntValue(5555)},
	{`toint(5555.05)`, value.NewIntValue(5555)},

	{`tobool("true")`, value.NewBoolValue(true)},
	{`tobool("t")`, value.NewBoolValue(true)},
	{`tobool("f")`, value.NewBoolValue(false)},
	{`tobool("hello")`, value.ErrValue},

	{`tonumber("5")`, value.NewNumberValue(float64(5))},
	{`tonumber("hello")`, value.ErrValue},
	{`tonumber("$ 5.22")`, value.NewNumberValue(float64(5.22))},
	{`tonumber("5.56")`, value.NewNumberValue(float64(5.56))},
	{`tonumber("$5.56")`, value.NewNumberValue(float64(5.56))},
	{`tonumber("5,555.00")`, value.NewNumberValue(float64(5555.00))},
	{`tonumber("€ 5,555.00")`, value.NewNumberValue(float64(5555.00))},

	/*
		Date functions
	*/

	{`seconds("M10:30")`, value.NewNumberValue(630)},
	{`seconds(replace("M10:30","M"))`, value.NewNumberValue(630)},
	{`seconds("M100:30")`, value.NewNumberValue(6030)},
	{`seconds("00:30")`, value.NewNumberValue(30)},
	{`seconds("30")`, value.NewNumberValue(30)},
	{`seconds(30)`, value.NewNumberValue(30)},
	{`seconds("2015/07/04")`, value.NewNumberValue(1435968000)},

	{`yy("10/13/2014")`, value.NewIntValue(14)},
	{`yy("01/02/2006")`, value.NewIntValue(6)},
	{`yy()`, value.NewIntValue(int64(ts.Year() - 2000))},

	{`mm("10/13/2014")`, value.NewIntValue(10)},
	{`mm("01/02/2006")`, value.NewIntValue(1)},

	{`yymm("10/13/2014")`, value.NewStringValue("1410")},
	{`yymm("01/02/2006")`, value.NewStringValue("0601")},

	{`hourofday("Apr 7, 2014 4:58:55 PM")`, value.NewIntValue(16)},
	{`hourofday()`, value.NewIntValue(16)},

	{`hourofweek("Apr 7, 2014 4:58:55 PM")`, value.NewIntValue(40)},

	{`totimestamp("Apr 7, 2014 4:58:55 PM")`, value.NewIntValue(1396889935)},

	{`extract(reg_date, "%B")`, value.NewStringValue("October")},
	{`extract(reg_date, "%d")`, value.NewStringValue("13")},
	{`extract("1257894000", "%B - %d")`, value.NewStringValue("November - 10")},
	{`extract("1257894000000", "%B - %d")`, value.NewStringValue("November - 10")},

	{`unixtrunc("1438445529707")`, value.NewStringValue("1438445529")},
	{`unixtrunc("1438445529", "ms")`, value.NewStringValue("1438445529000")},
	{`unixtrunc(todate(msdate))`, value.NewStringValue("1438445529")},
	{`unixtrunc(todate(msdate), "seconds")`, value.NewStringValue("1438445529.707")},
	{`unixtrunc(reg_date, "milliseconds")`, value.NewStringValue("1413158400000")},
	{`unixtrunc(reg_date, "seconds")`, value.NewStringValue("1413158400.0")},

	// Math
	{`pow(5,2)`, value.NewNumberValue(25)},
	{`pow(2,2)`, value.NewNumberValue(4)},
	{`pow(NotAField,2)`, value.ErrValue},
	{`pow(5,"hello")`, value.ErrValue},
	{`pow(5,"")`, value.ErrValue},

	{`sqrt(4)`, value.NewNumberValue(2)},
	{`sqrt(25)`, value.NewNumberValue(5)},
	{`sqrt(NotAField)`, value.ErrValue},
	{`sqrt("hello")`, value.ErrValue},

	// Aggregation functions
	{`sum(1,2)`, value.NewNumberValue(3)},
	{`sum(1,[2,3])`, value.NewNumberValue(6)},
	{`sum(1,"2")`, value.NewNumberValue(3)},
	{`sum(split("1,2", ","))`, value.NewNumberValue(3)},
	{`sum(["1","2"])`, value.NewNumberValue(3)},
	{`sum(["1","abc"])`, value.ErrValue},
	{`sum("hello")`, value.ErrValue},
	{`sum(exists("hello"))`, value.ErrValue},

	{`avg(1,2)`, value.NewNumberValue(1.5)},
	{`avg(1,[2,3])`, value.NewNumberValue(2.0)},
	{`avg(1,"2")`, value.NewNumberValue(1.5)},
	{`avg(["1","2"])`, value.NewNumberValue(1.5)},
	{`avg(["1","2","abc"])`, value.ErrValue},
	{`avg(split("1,2,3", ","))`, value.NewNumberValue(2.0)},
	{`avg(split("1,2,abc", ","))`, value.ErrValue},
	{`avg("hello")`, value.ErrValue},

	{`count(4)`, value.NewIntValue(1)},
	{`count(not_a_field)`, value.ErrValue},
	{`count(not_a_field)`, nil},

	// JsonPath
	{`json.jmespath(json, "[?name == 'n1'].name | [0]")`, value.NewStringValue("n1")},
	{`json.jmespath(json, "[?b].ct | [0]")`, value.NewNumberValue(8)},
	{`json.jmespath(json, "[?b].b | [0]")`, value.NewBoolValue(true)},
	{`json.jmespath(json, "[?b].tags | [0]")`, value.NewStringsValue([]string{"a", "b"})},
	{`json.jmespath(not_field, "[?b].tags | [0]")`, nil},
	{`json.jmespath(json, "[?b].tags | [0 ")`, nil},
}

var testValidation = []string{
	// math
	`sqrt()`,    // must have 1 args
	`sqrt(1,2)`, // must have 1 args
	`pow()`,     // must have 2 args
	`pow(1)`,    // must have 2 args
	// aggs
	`avg()`,                   // must have 1 args
	`sum()`,                   // must have 1 args
	`count()`, `count(a,b,c)`, // must have 1 arg

	// strings
	`contains()`, `contains(a,b,c)`, // must be 2 args
	`tolower()`, `tolower(a,b)`, // must be one arg
	`split()`, `split(a,",","hello")`, // must have 2 args
	`strip()`, `strip(a,"--")`, // must have 1 arg
	`replace(arg)`, `replace(arg,"with","replaceval","toomany")`, // must have 2 or 3 args
	`join("hello")`,
	`hasprefix()`, `hasprefix(a,b,"c")`, // 2 args
	`hassuffix()`, `hassuffix(a,b,"c")`, // 2 args

	`todatein("May 8, 2009 5:57:51 PM")`,                   // Must have 2 args
	`todatein("May 8, 2009 5:57:51 PM","PDT")`,             // PDT must be "America/Los_Angeles" format
	`todatein("May 8, 2009 5:57:51 PM","PDT","MORE")`,      // Too many args
	`todatein("May 8, 2009 5:57:51 PM", invalid_identity)`, // 2nd arg must be a string

	`json.jmespath(json)`,    // Must have 2 args
	`json.jmespath(json, 1)`, // Must have 2 args, 2nd must be string
}
var testValidationx = []string{
	`tolower()`, `lower(a,b)`, // must be one arg
}

func TestValidation(t *testing.T) {
	for _, exprText := range testValidation {
		_, err := expr.ParseExpression(exprText)
		assert.NotEqual(t, nil, err, exprText)
	}
}

func TestBuiltins(t *testing.T) {

	t1 := dateparse.MustParse("12/18/2015")
	nminus1 := time.Now().Add(time.Hour * -1)
	tr := true
	user := &User{
		Name:          "Yoda",
		Created:       t1,
		Updated:       &nminus1,
		Authenticated: true,
		HasSession:    &tr,
		Address:       Address{"Detroit", 55},
		Roles:         []string{"admin", "api"},
		BankAmount:    55.5,
		Hits:          map[string]int64{"foo": 5},
		FirstEvent:    map[string]time.Time{"signedup": t1},
	}
	readers := []expr.ContextReader{
		datasource.NewContextWrapper(user),
		readContext,
	}

	nc := datasource.NewNestedContextReader(readers, ts)

	for _, biTest := range builtinTests {

		//u.Debugf("expr:  %v", biTest.expr)
		exprNode, err := expr.ParseExpression(biTest.expr)
		assert.Equal(t, err, nil, "parse err: %v on %s", err, biTest.expr)

		val, ok := vm.Eval(nc, exprNode)
		if biTest.val == nil {
			assert.True(t, !ok, "Should not have evaluated? ok?%v val=%v", ok, val)
		} else if biTest.val.Err() {

			assert.True(t, !ok, "%v  expected err: %v", biTest.expr, ok)

		} else {

			assert.True(t, ok, "Should have evaluated: %s  %#v", biTest.expr, val)

			if fn, ok := exprNode.(*expr.FuncNode); ok {
				switch fn.F.Type() {
				case value.BoolType:
					assert.Equal(t, value.BoolType, val.Type())
				}
			}

			tval := biTest.val
			//u.Debugf("Type:  %T  %T", val, tval.Value)

			switch testVal := biTest.val.(type) {
			case nil:
				assert.True(t, !ok, "Not ok Get? %#v")
			case value.StringsValue:

				sa := tval.(value.StringsValue).Value().([]string)
				sb := val.Value().([]string)
				sort.Strings(sa)
				sort.Strings(sb)
				assert.True(t, strings.Join(sa, ",") == strings.Join(sb, ","),
					"should be == expect %v but was %v  %v", tval.Value(), val.Value(), biTest.expr)
			case value.MapValue:
				if len(testVal.Val()) == 0 {
					// we didn't expect it to work?
					_, ok := val.(value.MapValue)
					assert.True(t, !ok, "Was able to convert to mapvalue but should have failed %#v", val)
				} else {
					mv, ok := val.(value.MapValue)
					assert.True(t, ok, "Was able to convert to mapvalue: %#v", val)
					//u.Debugf("mv: %T  %v", mv, val)
					assert.True(t, len(testVal.Val()) == mv.Len(), "Should have same size maps")
					mivals := mv.Val()
					for k, v := range testVal.Val() {
						valVal := mivals[k]
						//u.Infof("k:%v  v:%v   valval:%v", k, v.Value(), valVal.Value())
						assert.Equal(t, valVal.Value(), v.Value(), "Must have found k/v:  %v \n\t%#v \n\t%#v", k, v, valVal)
					}
				}
			case value.Map:
				mv, ok := val.(value.Map)
				assert.True(t, ok, "Was able to convert to mapvalue: %#v", val)
				//u.Debugf("mv: %T  %v", mv, val)
				assert.True(t, testVal.Len() == mv.Len(), "Should have same size maps")
				mivals := mv.MapValue()
				for k, v := range testVal.MapValue().Val() {
					valVal, _ := mivals.Get(k)
					//u.Infof("k:%v  v:%v   valval:%v", k, v.Value(), valVal.Value())
					assert.Equal(t, valVal.Value(), v.Value(), "Must have found k/v:  %v \n\t%#v \n\t%#v", k, v, valVal)
				}
			case value.ByteSliceValue:
				assert.True(t, val.ToString() == tval.ToString(),
					"should be == expect %v but was %v  %v", tval.ToString(), val.ToString(), biTest.expr)
			case value.TimeValue:
				assert.Equal(t, val.Value(), val.Value())
			default:
				assert.True(t, val.Value() == tval.Value(),
					"should be == expect %v but was %v  %v", tval.Value(), val.Value(), biTest.expr)
			}
		}
	}
}
