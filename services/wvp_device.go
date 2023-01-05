package services

import (
	"ThingsPanel-Go/models"
	wvp "ThingsPanel-Go/others/wvp_http"
	uuid "ThingsPanel-Go/utils"
	valid "ThingsPanel-Go/validate"
	"errors"
	"strings"

	"github.com/beego/beego/v2/core/logs"
	"github.com/bitly/go-simplejson"
)

type WvpDeviceService struct {
}

type LoginInfo struct {
	Host     string
	Username string
	Password string
}

// 获取wvp服务地址和登录信息
func (*WvpDeviceService) GetWvpLoginInfo(protocolType string) (*LoginInfo, error) {
	var tpProtocolPluginService TpProtocolPluginService
	tpProtocolPlugin := tpProtocolPluginService.GetByProtocolType(protocolType, "2")
	var loginInfo LoginInfo
	loginInfoList := strings.Split(tpProtocolPlugin.HttpAddress, "||")
	if len(loginInfoList) == 3 {
		loginInfo.Host = loginInfoList[0]
		loginInfo.Username = loginInfoList[1]
		loginInfo.Password = loginInfoList[2]
	} else {
		return nil, errors.New("协议插件的http服务器地址配置有误,请按照{host}||{username}||{password}格式填写")
	}

	return &loginInfo, nil
}

// 如果是视频设备，修改完设备后再调用
func (*WvpDeviceService) AddSubVideoDevice(device valid.EditDevice) error {
	var deviceService DeviceService
	d, _ := deviceService.GetDeviceByID(device.ID)
	if d.DId == "" { // 设备编号为空退出
		return errors.New("设备编号不能为空")
	}
	count, err := deviceService.GetSubDeviceCount(device.ID)
	if err != nil {
		return err
	}
	if count > int64(0) { //有子设备退出
		return nil
	}
	// 地址http://119.91.238.241:18080，用户名admin，密码admin
	var WvpDeviceService WvpDeviceService
	LoginInfo, err := WvpDeviceService.GetWvpLoginInfo(d.Protocol)
	if err != nil {
		return err
	}
	// 根据协议类型在缓存中获取cookie
	// cookie := redis.GetStr(d.Protocol)
	// if cookie == "" {
	// 	//登录获取cookie
	// 	cookieValue, err := wvp.Login(LoginInfo.Host, LoginInfo.Username, LoginInfo.Password)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	cookie = cookieValue
	// 	redis.SetStr(d.Protocol, cookieValue, 3000*time.Second)
	// }
	//登录获取cookie
	logs.Info(LoginInfo.Host, LoginInfo.Username, LoginInfo.Password)
	cookieValue, err := wvp.Login(LoginInfo.Host, LoginInfo.Username, LoginInfo.Password)
	if err != nil {
		return err
	}
	logs.Info(cookieValue)
	cookie := cookieValue
	reqJson, err := wvp.GetDeviceChannels(LoginInfo.Host, d.DId, cookie)
	if err != nil {
		return err
	}
	if reqJson.Get("total").MustInt() < 1 { //失败
		if reqJson.Get("code").MustString() == "-1" {
			return errors.New(reqJson.Get("msg").MustString())
		} else {
			return errors.New("设备下没有开启的通道")
		}
	}
	logs.Info("获取通道列表成功")
	channelList, err := reqJson.Get("list").Array()
	if err != nil {
		return err
	}
	for _, channel := range channelList {
		logs.Info("channel:", channel)
		if channelMap, ok := channel.(map[string]interface{}); ok {
			channelId := channelMap["channelId"].(string)
			var additionalInfoJson simplejson.Json
			// 调接口查询播放地址
			var additionalInfo string
			logs.Info(LoginInfo.Host, d.DId, channelId, cookie)
			reqJson, err := wvp.GetPlayAddr(LoginInfo.Host, d.DId, channelId, cookie)
			if err == nil {
				logs.Info("获取播放地址成功：")
				additionalInfoJson.Set("video_address", reqJson.Get("data").Get("flv").MustString())
				additionalInfoByte, _ := additionalInfoJson.MarshalJSON()
				additionalInfo = string(additionalInfoByte)
			}
			var subDevice = models.Device{
				SubDeviceAddr:  channelId,
				Name:           channelMap["name"].(string),
				Protocol:       d.Protocol,
				ParentId:       d.ID,
				Token:          uuid.GetUuid(),
				DeviceType:     "3",
				AdditionalInfo: additionalInfo,
				AssetID:        d.AssetID,
			}
			logs.Info("开始添加子设备...")
			_, err = deviceService.Add(subDevice)
			if err != nil {
				logs.Error(err)
			}
		}
	}
	return nil
}

// ptz控制
func (*WvpDeviceService) PtzControl(parentId string, channelId string, queryMap map[string]string) error {
	var deviceService DeviceService
	d, _ := deviceService.GetDeviceByID(parentId)
	var WvpDeviceService WvpDeviceService
	LoginInfo, err := WvpDeviceService.GetWvpLoginInfo(d.Protocol)
	if err != nil {
		return err
	}
	// cookie := redis.GetStr(d.Protocol)
	// if cookie == "" {
	// 	//登录获取cookie
	// 	cookieValue, err := wvp.Login(LoginInfo.Host, LoginInfo.Username, LoginInfo.Password)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	cookie = cookieValue
	// 	redis.SetStr(d.Protocol, cookieValue, 3000*time.Second)
	// }
	//登录获取cookie
	cookieValue, err := wvp.Login(LoginInfo.Host, LoginInfo.Username, LoginInfo.Password)
	if err != nil {
		return err
	}
	cookie := cookieValue
	rsp, err := wvp.PtzControl(LoginInfo.Host, d.DId, channelId, cookie, queryMap)
	if err != nil {
		return err
	}
	if rsp != "success" {
		return errors.New(rsp)
	}
	return nil
}
