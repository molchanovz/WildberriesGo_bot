package bot

import (
	"context"
	"github.com/vmkteam/embedlog"
	"log"
	"tradebot/pkg/client/chatgptsrv"
	"tradebot/pkg/db"

	botlib "github.com/go-telegram/bot"
)

type Config struct {
	Token        string
	ReviewChatID int
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
	opts := []botlib.Option{botlib.WithDefaultHandler(s.manager.DefaultHandler)}
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
