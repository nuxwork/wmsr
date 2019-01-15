package main

import (
	"encoding/xml"
	"fmt"
	"image"
	"image/png"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/boringwork/wmsr/at"
)

// http://218.95.176.28:6080/arcgis/services/SHT_NX/SHT_4A/MapServer/WmsServer
// http://218.95.176.28:6080/arcgis/services/SHT_NX/SHT_4A/MapServer/WMSServer?request=GetCapabilities&service=WMS

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") //允许访问所有域
	w.Header().Set("Content-Type", "image/png")

	// fmt.Println("from: ", r.URL.String())
	query := r.URL.Query()
	url := getRedirect(query)
	// fmt.Println("  to: ", url)
	if url != "" {
		http.Redirect(w, r, url, 301)
	} else {
		png.Encode(w, EmptyPNG)
	}
}

func getRedirect(query url.Values) string {
	ret := ""
	bbox := strings.Split(query.Get("BBOX"), ",")
	b := BBox{}
	b.CRS = query.Get("CRS")
	b.MinX, _ = strconv.ParseFloat(bbox[0], 64)
	b.MinY, _ = strconv.ParseFloat(bbox[1], 64)
	b.MaxX, _ = strconv.ParseFloat(bbox[2], 64)
	b.MaxY, _ = strconv.ParseFloat(bbox[3], 64)

	c := []string{}
	for k, v := range WMSData {
		if !strings.EqualFold(b.CRS, k.CRS) {
			continue
		}

		if k.Contain(b.MinX, b.MinY) ||
			k.Contain(b.MaxX, b.MaxY) ||
			k.Contain(b.MaxX, b.MinY) ||
			k.Contain(b.MinX, b.MaxY) {
			ret = v
			break
		}

		if b.Contain(k.MinX, k.MinY) ||
			b.Contain(k.MaxX, k.MaxY) ||
			b.Contain(k.MaxX, k.MinY) ||
			b.Contain(k.MinX, k.MaxY) {
			ret = v
			break
		}

		if b.Contain(k.MinX, k.MinY) && b.Contain(k.MaxX, k.MaxY) {
			c = append(c, v)
		}
	}

	if ret == "" && len(c) >= 1 {
		ret = c[0]
	}

	// 替换参数
	if ret != "" {
		u, e := url.Parse(ret)
		if e != nil {
			fmt.Printf(`Error: Can't Parse %s, %s\n`, ret, e.Error())
			return ""
		}
		q := u.Query()
		_, hasLayer := q["layers"]

		for k, _ := range query {
			if !hasLayer && strings.EqualFold(k, "layers") {
				q.Set(k, query.Get(k))
			}
			if !strings.EqualFold(k, "version") && !strings.EqualFold(k, "layers") {
				q.Set(k, query.Get(k))
			}
		}
		u.RawQuery = q.Encode()
		ret = u.String()
	}

	return ret
}

func main() {
	if !PathExists("./config.at") {
		fmt.Printf(`Error: Can't found config.at file\n`)
		return
	}
	host, port, err := readConfig()
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("listen %s:%s\n", host, port)
	http.HandleFunc("/", IndexHandler)
	if err := http.ListenAndServe(fmt.Sprintf("%s:%s", host, port), nil); err != nil {
		fmt.Println(err)
	}
}

func readConfig() (host, port string, err error) {
	host = "0.0.0.0"
	port = "16080"

	configFile := filepath.Join(CurrentDirectory(), "config.at")
	bytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return
	}
	attr := at.Parse(string(bytes))
	host = attr.GetString("host", "0.0.0.0")
	port = attr.GetString("listen", "16080")
	wms := attr.GetArray("wms", make([]interface{}, 0))

	client := &http.Client{}
	for _, v := range wms {
		a := v.(at.Attribute)
		url := a.GetString("url", "")
		crs := a.GetString("crs", "")

		req, _ := http.NewRequest("GET", url, nil)
		q := req.URL.Query()
		q.Set("request", "GetCapabilities")
		q.Set("service", "WMS")
		req.URL.RawQuery = q.Encode()
		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("Error: %s when request %s\n", err.Error(), req.URL.String())
			continue
		}

		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error: %s when read response %s\n", err.Error(), req.URL.String())
			continue
		}

		cap := &Capabilities{}
		err = xml.Unmarshal(body, cap)
		if err != nil {
			fmt.Printf("Error: %s when Unmarshal %s\n", err.Error(), req.URL.String())
			continue
		}

		readBBox(crs, cap.Capability.Layers)
	}

	return
}

func readBBox(crs string, layers []Layer) {
	for _, layer := range layers {
		for _, bbox := range layer.BBox {
			if strings.EqualFold(crs, bbox.CRS) {
				u, e := url.QueryUnescape(layer.LegendURL.OnlineResource.Url)
				if e != nil {
					fmt.Printf("Error: %s when QueryUnescape %s\n", e.Error(), layer.LegendURL.OnlineResource.Url)
					continue
				}
				u, e = url.PathUnescape(u)
				if e != nil {
					fmt.Printf("Error: %s when PathUnescape %s\n", e.Error(), layer.LegendURL.OnlineResource.Url)
					continue
				}

				u2, e := url.Parse(u)
				if e != nil {
					fmt.Printf("Error: %s when Parse url %s\n", e.Error(), layer.LegendURL.OnlineResource.Url)
					continue
				}

				q := u2.Query()
				for k, _ := range q {
					if strings.EqualFold(k, "layer") {
						q.Set("layers", q.Get(k))
						q.Del(k)
					} else if !strings.EqualFold(k, "version") {
						q.Del(k)
					}
				}

				u2.RawQuery = q.Encode()

				WMSData[bbox] = u2.String()
				break
			}
		}
	}
}

func CurrentDirectory() string {
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Printf("Error: %s when get CurrentDirectory\n", err.Error())
	}
	return strings.Replace(dir, "\\", "/", -1)
}

func PathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if os.IsNotExist(err) {
		return false
	}
	return false
}

type Point struct {
	X float64
	Y float64
}

type Capabilities struct {
	XMLName    xml.Name   `xml:"WMS_Capabilities"`
	Version    string     `xml:"version,attr"`
	Service    Service    `xml:"Service"`
	Capability Capability `xml:"Capability"`
}

type Service struct {
	Name string `xml:"Name"`
}

type Capability struct {
	Layers []Layer `xml:"Layer>Layer"`
}

type Layer struct {
	Name      string    `xml:"Name"`
	BBox      []BBox    `xml:"BoundingBox"`
	LegendURL LegendURL `xml:"Style>LegendURL"`
}

type BBox struct {
	CRS  string  `xml:"CRS,attr"`
	MinX float64 `xml:"minx,attr"`
	MinY float64 `xml:"miny,attr"`
	MaxX float64 `xml:"maxx,attr"`
	MaxY float64 `xml:"maxy,attr"`
}

func (me BBox) Contain(x, y float64) bool {
	return x >= me.MinX && y >= me.MinY && x <= me.MaxX && y <= me.MaxY
}

type LegendURL struct {
	Width          int            `xml:"width,attr"`
	Height         int            `xml:"height,attr"`
	Format         string         `xml:"Format"`
	OnlineResource OnlineResource `xml:"OnlineResource"`
}

type OnlineResource struct {
	Url string `xml:"http://www.w3.org/1999/xlink href,attr"`
}

var WMSData map[BBox]string = map[BBox]string{}
var EmptyPNG image.Image = image.NewRGBA(image.Rect(0, 0, 1, 1))
