package bot

import (
	"context"
	"log"
	"net/http"
	"net/url"
	"time"

	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/db"

	botlib "github.com/go-telegram/bot"
	"github.com/vmkteam/embedlog"
)

type Config struct {
	Token        string
	ReviewChatID int
	ProxyURL     string
}

type Service struct {
	cfg     Config
	manager *Manager
}

func NewService(cfg Config, dbc db.DB, chatgpt *chatgptsrv.Client, logger embedlog.Logger) *Service {
	return &Service{
		cfg:     cfg,
		manager: NewManager(dbc, cfg, chatgpt, logger),
	}
}

func (s *Service) Manager() *Manager {
	return s.manager
}

func (s *Service) Start() {
	opts := []botlib.Option{botlib.WithDefaultHandler(s.manager.DefaultHandler), botlib.WithCheckInitTimeout(15 * time.Second)}
	if s.cfg.ProxyURL != "" {
		proxyURL, err := url.Parse(s.cfg.ProxyURL)
		if err != nil {
			log.Printf("ошибка парсинга прокси: %v", err)
		} else {
			httpClient := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
			opts = append(opts, botlib.WithHTTPClient(60*time.Second, httpClient))
			log.Printf("Используется прокси: %s", s.cfg.ProxyURL)
		}
	}
	newBot, err := botlib.New(s.cfg.Token, opts...)
	if err != nil {
		log.Printf("ошибка запуска бота: %v", err)
		return
	}
	s.manager.SetBot(newBot)
	go func() {
		log.Printf("Бот запущен\n")
		newBot.Start(context.Background())
	}()
	s.manager.RegisterBotHandlers()
}
