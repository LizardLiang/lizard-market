module.exports = {
  apps: [
    {
      name: "discord-orchestrator",
      script: "bun",
      args: "run server.ts",
      interpreter: "none",
      cwd: __dirname,
      watch: false,
      autorestart: true,
      max_restarts: 10,
      restart_delay: 5000,
      env: {
        NODE_ENV: "production",
      },
    },
  ],
};
