# Goal of the Code Trainer

Code Trainer is a gamified way to learn a programming language by building a real project.

## Audience

- **First version:** intermediate developers who already program and want to learn a specific language. The first languages are **Rust** and **Go**.
- **Later phase:** a beginner course that teaches programming itself. It is out of scope for now.

## Learning approach

- **Learn by building.** The learner picks one project from several offered for each language. The projects are systems projects (see [Projects](#projects)). Knowledge is introduced as the project needs it: language features, best practices, code smells and architectural decisions.
- **Architecture inside the project.** The learner faces real design choices as the project grows, such as how to structure modules or layers, with short explanations of the trade-offs.
- **Learning from mistakes.** Learners are allowed to run into mistakes. Automated tests and checks give feedback that explains what went wrong and how to fix it.
- **Bad-code reviews.** Lessons include examples of bad code. The learner reviews them, works out why the code is bad and improves it.
- **Fixed difficulty path.** Lessons follow a fixed order from easy to hard, so difficulty grows with the learner's experience.

## Experience

- **Gamified.** The trainer uses XP, levels and badges, daily streaks and goals, a story or game world around the project, and leaderboards and other social features.
- **Fun and engaging.** The gamification elements above serve this goal.
- **Small daily lessons.** A lesson takes about 5 minutes, so learners can make steady progress without long courses. Each lesson is one small coding change, and a larger feature spans several lessons over several days.

## Where learning happens

- **In the IDE.** An IDE plugin delivers lessons, tasks and feedback inside the editor. It supports VS Code, GoLand, RustRover and IntelliJ IDEA. Learners get used to the tools they will use in real work as well as the language.
- **On the go.** Mobile and other devices offer quizzes and flashcards and code review exercises of bad code. Progress syncs with the IDE plugin through one account.

## Projects

Each language offers five systems projects. The first version ships two of them.

| Project | Rust | Go | First version |
|---|---|---|---|
| Redis-style server | yes | yes | yes |
| HTTP server | yes | yes | yes |
| Shell | yes | yes | later |
| Memory allocator | yes | | later |
| Parser / interpreter | yes | | later |
| Concurrent job queue | | yes | later |
| Reverse proxy / load balancer | | yes | later |
