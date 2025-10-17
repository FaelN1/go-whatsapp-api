const os = require('os');
const path = require('path');

// Detecta o sistema operacional
const isWindows = os.platform() === 'win32';
const binaryName = isWindows ? 'server.exe' : 'server';
const binaryPath = path.join('.', 'bin', binaryName);

module.exports = {
  apps: [
    {
      name: 'go-whatsapp-api',
      script: binaryPath,
      cwd: './',
      instances: 1,
      exec_mode: 'fork',
      autorestart: true,
      watch: false,
      max_memory_restart: '1G',
      env: {
        NODE_ENV: 'production',
        // Variáveis de ambiente são carregadas do .env pelo código Go
      },
      env_production: {
        NODE_ENV: 'production',
      },
      env_development: {
        NODE_ENV: 'development',
      },
      error_file: './logs/err.log',
      out_file: './logs/out.log',
      log_file: './logs/combined.log',
      time: true,
      merge_logs: true,
      // Restart delay
      restart_delay: 4000,
      min_uptime: 5000,
      max_restarts: 10,
      // Kill timeout
      kill_timeout: 5000,
      // Listen timeout
      listen_timeout: 3000,
      // Configurações de log
      log_date_format: 'YYYY-MM-DD HH:mm:ss Z',
      combine_logs: true,
    },
  ],
};
