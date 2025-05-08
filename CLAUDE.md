# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Japanese Language Learning App in Telegram Mini Apps
The web-interface app (telegram miniapp), similar to Anki, includes:

- Social feature: tracking user progress through a feed/leaderboard.

- Flashcards: sentence examples, audio, furigana for words and examples.

- Practice: listening and sentence composition with AI-checking.

- MVP: decks for N5, N4, N3 from exam materials.

- Post-MVP: adding custom cards with AI-generated content.

Focus on usability, speed, and content quality.

## Architecture

- **cmd/api/main.go**: Entry point that initializes the server, connects to the database, and sets up routes
- **internal/db**: Database layer for user management and other data operations
- **internal/handlers**: Request handlers for the HTTP API and Telegram bot
- **internal/middleware**: Echo middleware configuration for logging, auth, and more
- **materials**: Contains Japanese vocabulary and grammar resources (JSON files)
- **nternal/ai**: TODO: AI for generation/checking

## Key Components

- **Echo Framework**: HTTP server and routing
- **SQLite**: Database backend
- **Telegram Bot API**: Integration with Telegram
- **JWT**: Authentication for API endpoints
- **SolidJS**: TODO: Frontend framework for the telegram miniapp

## Important Notes

- Comments are added only when the code's purpose isn't obvious - not meta-comments that merely rephrase the function
  name or explain an 'if' condition that’s already clear from the code.