package service_test

import (
	"danmo-work/core/port"
	"danmo-work/core/service"
	sqlitestore "danmo-work/core/store/sqlite"
)

func testIngress(st *sqlitestore.Store, sessions *service.SessionManager, pm *service.ProjectManager, cm *service.ConfigManager) *service.ChannelIngressService {
	peers := service.NewMultiplexPeerStore(map[port.ChannelType]port.ChannelPeerStore{
		port.ChannelWeixin: service.NewWeixinPeerStore(st),
		port.ChannelFeishu: service.NewFeishuPeerStore(st, cm),
		port.ChannelWecom:  service.NewWecomPeerStore(st, cm),
	})
	return service.NewChannelIngress(sessions, pm, peers, service.NewConfigChannelDefaults(cm))
}

func testWeixinBridge(st *sqlitestore.Store, pm *service.ProjectManager, cm *service.ConfigManager) *service.WeixinBridge {
	sessions := service.NewSessionManager(st, nil, nil)
	return service.NewWeixinBridge(st, sessions, pm, cm, testIngress(st, sessions, pm, cm))
}
