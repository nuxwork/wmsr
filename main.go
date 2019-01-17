package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	"image/draw"
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

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*") //允许访问所有域
	w.Header().Set("Content-Type", "image/png")

	// fmt.Println("from: ", r.URL.String())
	query := r.URL.Query()
	ret := getRedirect(query)
	// fmt.Println("  to: ", url)
	if len(ret) == 0 {
		png.Encode(w, EmptyPNG)
	} else if len(ret) == 1 {
		http.Redirect(w, r, ret[0], 301)
	} else if len(ret) > 1 {
		png.Encode(w, mergeImages(ret))
	}
}

func mergeImages(urls []string) *image.RGBA {
	l := len(urls)
	ch := make(chan []byte, l)
	for _, u := range urls {
		go func(s string) {
			resp, err := http.Get(s)
			if err != nil {
				fmt.Printf(`Error: request "%s", %s`, s, err.Error())
				ch <- nil
				return
			} else {
				defer resp.Body.Close()
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					fmt.Printf("Error: %s when read response %s\n", err.Error(), s)
					ch <- nil
					return
				}

				// fmt.Println("request done")
				ch <- body
			}

		}(u)
	}

	var rgba *image.RGBA = nil
	var bounds image.Rectangle

	for i := 0; i != l; i++ {
		imgBytes := <-ch
		if imgBytes == nil {
			continue
		}

		img, _, err := image.Decode(bytes.NewReader(imgBytes))
		if err != nil {
			fmt.Printf("Error: %s when decode image %s\n", err.Error())
			continue
		}

		if rgba == nil {
			bounds = img.Bounds()
			rgba = image.NewRGBA(bounds)
			draw.Draw(rgba, bounds, img, image.Point{0, 0}, draw.Src)
		}

		draw.Draw(rgba, bounds, img, image.Point{0, 0}, draw.Over)

	}

	return rgba
}

func getRedirect(query url.Values) []string {
	res := []string{}
	bbox := strings.Split(query.Get("BBOX"), ",")
	b := BBox{}
	b.CRS = query.Get("CRS")
	b.MinX, _ = strconv.ParseFloat(bbox[0], 64)
	b.MinY, _ = strconv.ParseFloat(bbox[1], 64)
	b.MaxX, _ = strconv.ParseFloat(bbox[2], 64)
	b.MaxY, _ = strconv.ParseFloat(bbox[3], 64)

	for k, v := range WMSData {
		if !strings.EqualFold(b.CRS, k.CRS) {
			continue
		}

		// 任何一个请求点在景区内
		if k.Contain(b.MinX, b.MinY) ||
			k.Contain(b.MaxX, b.MaxY) ||
			k.Contain(b.MaxX, b.MinY) ||
			k.Contain(b.MinX, b.MaxY) {
			res = append(res, v)
		}

		// 景区任何一个点在请求范围内
		if b.Contain(k.MinX, k.MinY) ||
			b.Contain(k.MaxX, k.MaxY) ||
			b.Contain(k.MaxX, k.MinY) ||
			b.Contain(k.MinX, k.MaxY) {
			res = append(res, v)
		}
	}

	ret := []string{}
	if len(res) > 0 {
		for _, s := range res {
			u, e := url.Parse(s)
			if e != nil {
				fmt.Printf(`Error: Can't Parse %s, %s\n`, s, e.Error())
				continue
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
			ret = append(ret, u.String())
		}
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
