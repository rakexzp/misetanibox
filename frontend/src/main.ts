import { createApp } from 'vue'
import App from './App.vue'
import './style.css'
import { initStore } from './store'
import { initColors } from './utils/colors'
import { initPerf } from './utils/perf'
import { EventsEmit } from '../wailsjs/runtime/runtime'

const setupConsoleInterception = () => {
  const originalConsole = {
    log: console.log,
    info: console.info,
    warn: console.warn,
    error: console.error,
    debug: console.debug
  };

  const padZero = (n: number) => n.toString().padStart(2, '0');
  const getTimeStr = () => {
    const d = new Date();
    return `${padZero(d.getHours())}:${padZero(d.getMinutes())}:${padZero(d.getSeconds())}`;
  };

  const formatArgs = (args: any[]) => {
    return args.map(arg => {
      if (arg instanceof Error) {
        return arg.stack || arg.message || String(arg);
      }
      if (typeof arg === 'object' && arg !== null) {
        try {
          const str = JSON.stringify(arg);
          return str === '{}' ? String(arg) : str;
        } catch (e) {
          return String(arg);
        }
      }
      return String(arg);
    }).join(' ');
  };

  const emitLog = (type: string, args: any[]) => {
    const payload = formatArgs(args);
    EventsEmit("log-message", {
      type,
      payload,
      time: getTimeStr()
    });
  };

  console.log = function (...args) {
    originalConsole.log.apply(console, args);
    emitLog('info', args);
  };
  console.info = function (...args) {
    originalConsole.info.apply(console, args);
    emitLog('info', args);
  };
  console.warn = function (...args) {
    originalConsole.warn.apply(console, args);
    emitLog('warning', args); // clash/内核的 type 可能是 warning 或者是 warn，这里统一推 warning
  };
  console.error = function (...args) {
    originalConsole.error.apply(console, args);
    emitLog('error', args);
  };
  console.debug = function (...args) {
    originalConsole.debug.apply(console, args);
    emitLog('debug', args);
  };
};

setupConsoleInterception();
initStore(); // 启动全局监听
initColors(); // применить пользовательскую покраску
initPerf(); // экономия GPU: пауза анимаций в трее + эконом-режим

createApp(App).mount('#app')