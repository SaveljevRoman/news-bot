package main

import (
	"context"
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"log"
	"news-bot/internal/bot"
	"news-bot/internal/bot/middleware"
	"news-bot/internal/botkit"
	"news-bot/internal/config"
	fetcher2 "news-bot/internal/fetcher"
	notifier2 "news-bot/internal/notifier"
	"news-bot/internal/storage"
	"news-bot/internal/summary"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	botAPI, err := tgbotapi.NewBotAPI(config.Get().TelegramBotToken)
	if err != nil {
		//используем простой log вместо fatal что бы отработали отложенные ф-ии
		log.Printf("failed to create bot: %v", err)
		return
	}

	db, err := sqlx.Connect("postgres", config.Get().DatabaseDSN)
	if err != nil {
		log.Printf("failed to connect database: %v", err)
		return
	}
	defer db.Close()

	var (
		articleStorage = storage.NewArticleStorage(db)
		sourceStorage  = storage.NewSourceStorage(db)
		fetcher        = fetcher2.New(
			articleStorage,
			sourceStorage,
			config.Get().FetchInterval,
			config.Get().FilterKeywords,
		)
		notifier = notifier2.New(
			articleStorage,
			summary.NewOpenAISummarizer(config.Get().OpenAIKey, config.Get().OpenAIPrompt),
			botAPI,
			config.Get().NotificationInterval,
			2*config.Get().FetchInterval,
			config.Get().TelegramChannelID,
		)
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	newsBot := botkit.New(botAPI)
	newsBot.RegisterCmdView("start", bot.ViewCmStart())
	newsBot.RegisterCmdView("addsource", middleware.AdminsOnly(config.Get().TelegramChannelID, bot.ViewCmdAddSource(sourceStorage)))
	newsBot.RegisterCmdView("listsources", middleware.AdminsOnly(config.Get().TelegramChannelID, bot.ViewCmdListSources(sourceStorage)))
	newsBot.RegisterCmdView("deletesource", middleware.AdminsOnly(config.Get().TelegramChannelID, bot.ViewCmdDeleteSource(sourceStorage)))

	go func(ctx context.Context) {
		if err := fetcher.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("[ERROR] failed to start fetcher: %v", err)
				return
			}
			log.Println("fetcher stopped")
		}
	}(ctx)

	go func(ctx context.Context) {
		if err := notifier.Start(ctx); err != nil {
			if !errors.Is(err, context.Canceled) {
				log.Printf("[ERROR] failed to start notifier: %v", err)
				return
			}
			log.Println("notifier stopped")
		}
	}(ctx)

	if err := newsBot.Run(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			log.Printf("[ERROR] failed to run bot: %v", err)
			return
		}
		log.Println("notifier stopped")
	}
}
