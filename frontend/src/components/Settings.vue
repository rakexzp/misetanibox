<template>
  <div class="settings-container">

    <Transition name="slide-fade" mode="out-in">
      <div :key="view" class="settings-view-wrapper">

        <div v-if="view === 'main'" class="settings-page">
          <div class="glass-card setting-group">
            <h3>Настройки сети</h3>

            <div class="setting-item clickable" @click="view = 'network'">
              <div class="info">
                <h4>Базовые настройки сети</h4>
                <p>TCP-конкурентность ядра, таймауты и логика проверки соединений.</p>
              </div>
              <span class="arrow">➔</span>
            </div>

            <div class="setting-item clickable" @click="view = 'dns'">
              <div class="info">
                <h4>Конфигурация DNS-серверов</h4>
                <p>Антиспуфинг-резолвинг, политика Fake-IP и DNS-группы для маршрутизации.</p>
              </div>
              <span class="arrow">➔</span>
            </div>

            <div class="setting-item clickable" @click="view = 'tun'">
              <div class="info">
                <h4>Виртуальный адаптер (Режим TUN)</h4>
                <p>Виртуальный сетевой адаптер — прозрачный прокси для всего трафика.</p>
              </div>
              <span class="arrow">➔</span>
            </div>
          </div>

          <div class="glass-card setting-group">
            <h3>Оформление — цвета</h3>
            <div class="color-row">
              <label class="color-swatch" :style="{ background: colors.accent }">
                <input type="color" v-model="colors.accent" @input="onColorInput" />
              </label>
              <div class="info"><h4>Акцент</h4><p>Выделения, активные элементы, индикаторы.</p></div>
            </div>
            <div class="color-row">
              <label class="color-swatch" :style="{ background: colors.text }">
                <input type="color" v-model="colors.text" @input="onColorInput" />
              </label>
              <div class="info"><h4>Текст (буквы)</h4><p>Основной цвет текста интерфейса.</p></div>
            </div>
            <div class="color-row">
              <label class="color-swatch" :style="{ background: colors.bg }">
                <input type="color" v-model="colors.bg" @input="onColorInput" />
              </label>
              <div class="info"><h4>Фон</h4><p>Фон приложения и панелей (оттенки выводятся автоматически).</p></div>
            </div>
            <div class="color-presets">
              <button v-for="p in COLOR_PRESETS" :key="p.name" class="color-preset"
                      :style="{ background: p.bg, color: p.text, borderColor: p.accent }"
                      @click="applyColorPreset(p)">{{ p.name }}</button>
            </div>
            <div class="setting-item" style="border:none;padding-top:4px">
              <div class="info"><p>Цвета применяются поверх светлой/тёмной темы. Скоро — публикация в Мастерскую.</p></div>
              <button class="action-btn" @click="resetColors">Сбросить</button>
            </div>
            <div class="setting-item">
              <div class="info">
                <h4>Экономный режим анимаций</h4>
                <p>Отключает волну трафика и свечение орба — меньше нагрузки на видеокарту. Пригодится, если приложение греет GPU.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" :checked="economy" @change="toggleEconomy">
                <span class="slider"></span>
              </label>
            </div>
            <div class="setting-item" v-if="isWindows">
              <div class="info">
                <h4>Разгрузка GPU интерфейса</h4>
                <p>Отключает GPU-ускорение вебвью — интерфейс рисуется через CPU. Помогает, если видеокарта греется именно от приложения. Применяется после перезапуска.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" :checked="gpuSaver" @change="toggleGpuSaver">
                <span class="slider"></span>
              </label>
            </div>
          </div>

          <div class="glass-card setting-group">
            <h3>Настройки приложения</h3>

            <div class="setting-item clickable" @click="view = 'behavior'">
              <div class="info">
                <h4>Поведение приложения</h4>
                <p>Режим запуска, логика трея и User-Agent запросов подписки.</p>
              </div>
              <span class="arrow">➔</span>
            </div>

            <div class="setting-item clickable" v-if="isWindows" @click="enterUwpManager">
              <div class="info">
                <h4>UWP loopback-исключения</h4>
                <p>Доступ UWP-приложений (Store, Почта) к локальному прокси.</p>
              </div>
              <span class="arrow">➔</span>
            </div>

            <div class="setting-item clickable" @click="view = 'update'">
              <div class="info">
                <h4>Обновление компонентов и баз</h4>
                <p>Ядро Mihomo, драйвер Wintun и базы правил GeoIP/GeoSite.</p>
              </div>
              <span class="arrow">➔</span>
            </div>

            <div class="setting-item clickable" @click="view = 'about'">
              <div class="info">
                <h4>О приложении</h4>
                <p>Версия, резервная копия и восстановление, репозиторий GitHub.</p>
              </div>
              <span class="arrow">➔</span>
            </div>
          </div>
        </div>

        <div v-else-if="view === 'update'" class="settings-page">
          <div class="sub-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <span class="icon back-icon-svg" v-html="ICONS.arrowLeft"></span>
            </button>
            <h3>Обновление компонентов и баз</h3>
          </div>

          <UpdateTaskPanel />

          <div class="glass-card setting-group scrollable">
            <div class="setting-item col-item" style="padding-bottom: 0; align-items: flex-start;">
              <h3 style="margin: 0; font-size: 1.15rem; font-weight: 600; color: var(--text-main);">Ядро и драйвер</h3>
            </div>
            <div class="divider" style="margin-top: 10px;"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Ядро Mihomo <span style="color: var(--accent); margin-left: 8px; font-style: italic; font-size: 0.8rem; font-weight: normal;">(обновление кратко разорвёт прокси)</span></h4>
                <p>Текущая версия: {{ coreVersion }}</p>
              </div>
              <button 
                class="action-btn" 
                :class="{ 'accent-btn': globalState.componentUpdate.pendingCoreUpdate }"
                @click="globalState.componentUpdate.pendingCoreUpdate ? executeCoreUpdate() : handleUpdateCore()" 
                :disabled="globalState.componentUpdate.checkingCoreUpdate || globalState.componentUpdate.updatingCore"
              >
                <template v-if="globalState.componentUpdate.checkingCoreUpdate">Проверка…</template>
                <template v-else-if="globalState.componentUpdate.updatingCore">Обработка…</template>
                <template v-else-if="globalState.componentUpdate.pendingCoreUpdate">Обновить до {{ globalState.componentUpdate.coreUpdateInfo.remote }}</template>
                <template v-else>Проверить обновления</template>
              </button>
            </div>

            <div class="divider" v-if="isWindows"></div>

            <div class="setting-item" v-if="isWindows">
              <div class="info">
                <h4>Драйвер Wintun (DLL)</h4>
                <p>Текущая версия: {{ wintunVersion || 'Получение…' }}</p>
              </div>
              <div class="btn-group">
                <button class="action-btn" @click="installDriver(true)" :disabled="isInstalling">
                  {{ isInstalling ? 'Обработка…' : 'Переустановить' }}
                </button>
              </div>
            </div>
            
            <div class="divider"></div>

            <div class="setting-item" v-if="isWindows">
              <div class="info">
                <h4>Фоновая служба (GoclashZHelper)</h4>
                <p>
                  Статус:
                  <span :style="{ color: helperStatus.reachable ? 'var(--green-text)' : (helperStatus.installed ? 'var(--yellow-text)' : 'var(--text-muted)') }">
                    {{ helperStatus.reachable ? 'Работает' : (helperStatus.installed ? (helperStatus.running ? 'Нет связи' : 'Остановлена') : 'Не установлена') }}
                  </span>
                  <span v-if="!helperStatus.installed" style="color: var(--text-muted); font-size: 0.75rem;"> · нужна для автозапуска TUN</span>
                </p>
              </div>
              <div class="btn-group">
                <button class="action-btn" @click="restartHelper" :disabled="helperLoading" v-if="helperStatus.installed">
                  {{ helperLoading ? '...' : 'Перезапустить' }}
                </button>
                <button class="action-btn" @click="installHelper" :disabled="helperLoading">
                  {{ helperLoading ? '...' : (helperStatus.installed ? 'Переустановить' : 'Установить') }}
                </button>
              </div>
            </div>

            <div class="divider"></div>

            <div class="setting-item col-item" style="flex-direction: row; justify-content: space-between; align-items: center; padding-bottom: 0; margin-top: 10px;">
              <div class="info">
                <h3 style="margin: 0; font-size: 1.15rem; font-weight: 600; color: var(--text-main);">Базы правил маршрутизации</h3>
              </div>
              <button class="action-btn primary-btn accent-btn" @click="handleUpdateAllDbs" :disabled="isUpdatingAnyDb">
                {{ isUpdatingAnyDb ? 'Обновление…' : 'Обновить все' }}
              </button>
            </div>
            <div class="divider" style="margin-top: 14px;"></div>

            <template v-for="(db, idx) in dbList" :key="db.key">
              <div class="setting-item">
                <div class="info" style="overflow: hidden;">
                  <h4>Файл {{ db.title }}</h4>
                  <p class="link-text" style="white-space: nowrap; overflow: hidden; text-overflow: ellipsis;">
                    {{ behavior[db.behaviorKey] || 'Ссылка не задана' }}
                  </p>
                  <p v-if="dbFileInfo[db.key]?.ready" style="font-size: 0.75rem; color: var(--text-muted); margin-top: 2px;">
                    Размер: {{ formatBytes(dbFileInfo[db.key].size) }} | Обновлено: {{ formatRelativeTime(dbFileInfo[db.key].modTime) }}
                  </p>
                  <p v-else-if="dbFileInfo[db.key]?.error" style="font-size: 0.75rem; color: var(--red-text); margin-top: 2px;">{{ dbFileInfo[db.key].error }}</p>
                  <p v-else style="font-size: 0.75rem; color: var(--red-text); margin-top: 2px;">Файла нет — нажмите «Синхронизировать»</p>
                </div>
                <div class="btn-group" style="flex-shrink: 0;">
                  <button class="action-btn" @click="openDbEditModal(db.key, behavior[db.behaviorKey])" :disabled="isUpdatingDb(db.key)">Изменить ссылку</button>
                  <button class="action-btn" @click="handleUpdateDb(db.key)" :disabled="isUpdatingDb(db.key)">
                    {{ isUpdatingDb(db.key) ? 'Синхронизация…' : 'Синхронизировать' }}
                  </button>
                </div>
              </div>
              <div class="divider" v-if="idx < dbList.length - 1"></div>
            </template>
          </div>
        </div>

        <div v-else-if="view === 'tun'" class="settings-page">
          <div class="sub-header section-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
            </button>
            <h3>Виртуальный адаптер</h3>
            <button class="action-btn accent-btn mini-btn-reset" @click="confirmReset('tun')">
              <span class="btn-icon" v-html="ICONS.refresh"></span> Сброс
            </button>
          </div>

          <div class="glass-card setting-group scrollable">

            <div class="setting-item">
              <div class="info"><h4>Включить режим TUN</h4></div>
              <label class="modern-switch">
                <input type="checkbox" :checked="globalState.tun" @change="handleTunToggle">
                <span class="slider"></span>
              </label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Установка драйвера адаптера</h4>
                <p class="status-msg">
                  Статус проверки: <span :class="tunStatus.hasWintun ? 'green-text' : 'red-text'">{{ tunStatus.hasWintun ? 'wintun готов' : (tunStatus.wintunError || 'wintun недоступен') }}</span>
                </p>
              </div>
              <button class="action-btn" @click="installDriver(true)" :disabled="isInstalling || tunStatus.hasWintun">
                {{ isInstalling ? 'Обработка…' : (tunStatus.hasWintun ? 'Установлен' : 'Установить драйвер') }}
              </button>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Стек (Stack)</h4>
                <p>{{ stackHint }}</p>
              </div>
              <ModernSelect
                v-model="tunConfig.stack"
                :options="stackOptions"
                @change="saveTun"
                :disabled="!tunStatus.hasWintun"
              />
            </div>

            <div class="divider" v-if="isLinux"></div>

            <div class="setting-item" v-if="isLinux">
              <div class="info">
                <h4>Ускорение (GSO)</h4>
                <p>Сегментация пакетов на стороне ядра — поднимает скорость TUN. Применяется при переподключении.</p>
              </div>
              <label class="modern-switch"><input type="checkbox" v-model="tunConfig.gso" @change="saveTun"><span class="slider"></span></label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Имя адаптера (Device)</h4></div>
              <input type="text" class="modern-input" v-model="tunConfig.device" placeholder="Пусто = авто" @blur="saveTun" :disabled="!tunStatus.hasWintun" />
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Авто-маршруты (Auto Route)</h4></div>
              <label class="modern-switch"><input type="checkbox" v-model="tunConfig.autoRoute" @change="saveTun" :disabled="!tunStatus.hasWintun"><span class="slider"></span></label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Автоопределение интерфейса (Auto Detect Interface)</h4></div>
              <label class="modern-switch"><input type="checkbox" v-model="tunConfig.autoDetect" @change="saveTun" :disabled="!tunStatus.hasWintun"><span class="slider"></span></label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Перехват DNS (DNS Hijack)</h4></div>
              <input type="text" class="modern-input" :value="tunConfig.dnsHijack.join(', ')" @blur="updateTunDnsHijack" placeholder="напр. any:53" :disabled="!tunStatus.hasWintun" />
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Строгие маршруты (Strict Route)</h4></div>
              <label class="modern-switch"><input type="checkbox" v-model="tunConfig.strictRoute" @change="saveTun" :disabled="!tunStatus.hasWintun"><span class="slider"></span></label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>MTU</h4></div>
              <ModernNumberInput 
                v-model="tunConfig.mtu" 
                :min="576" 
                :max="1500" 
                @change="saveTun" 
                :disabled="!tunStatus.hasWintun" 
              />
            </div>

          </div>
        </div>

        <div v-else-if="view === 'dns'" class="settings-page">
          <div class="sub-header section-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
            </button>
            <h3>Конфигурация DNS-серверов</h3>
            <button class="action-btn accent-btn mini-btn-reset" @click="confirmReset('dns')">
              <span class="btn-icon" v-html="ICONS.refresh"></span> Сброс
            </button>
          </div>

          <div class="glass-card setting-group scrollable">

            <div class="setting-item">
              <div class="info"><h4>Переопределение DNS (Enable DNS)</h4></div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.enable" @change="saveDns">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Порт DNS (Listen)</h4></div>
              <input type="text" class="modern-input" v-model="dnsConfig.listen" @blur="saveDns" :disabled="!dnsConfig.enable" placeholder="напр. 0.0.0.0:1053" />
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Резолвинг IPv6 (IPv6 Resolution)</h4></div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.ipv6" @change="saveDns" :disabled="!dnsConfig.enable">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Предпочитать HTTP/3 (Prefer HTTP/3)</h4>
                <p>Серверы с DoH3 сначала по HTTP/3</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.preferH3" @change="saveDns" :disabled="!dnsConfig.enable">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Расширенный режим (Enhanced Mode)</h4></div>
              <ModernSelect 
                v-model="dnsConfig.enhancedMode" 
                :options="enhancedModeOptions" 
                @change="saveDns" 
                :disabled="!dnsConfig.enable" 
              />
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Учитывать правила (Respect Rules)</h4>
                <p>В режиме Fake-IP правила решают, вернуть ли реальный IP</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.respectRules" @change="saveDns" :disabled="!dnsConfig.enable || dnsConfig.enhancedMode !== 'fake-ip'">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Диапазон Fake-IP (Fake-IP Range)</h4></div>
              <input type="text" class="modern-input" v-model="dnsConfig.fakeIpRange" @blur="saveDns" :disabled="!dnsConfig.enable || dnsConfig.enhancedMode !== 'fake-ip'" />
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>Фильтр Fake-IP (Fake-IP Filter)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.fakeIpFilter || []).join('\n')" @blur="updateDnsArray($event, 'fakeIpFilter')" rows="3" placeholder="напр. *.lan" :disabled="!dnsConfig.enable || dnsConfig.enhancedMode !== 'fake-ip'"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Системный hosts (Use System Hosts)</h4></div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.useSystemHosts" @change="saveDns" :disabled="!dnsConfig.enable">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Использовать hosts (Use Hosts)</h4></div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.useHosts" @change="saveDns" :disabled="!dnsConfig.enable">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>DNS по умолчанию (Default Nameservers)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.defaultNameserver || []).join('\n')" @blur="updateDnsArray($event, 'defaultNameserver')" rows="2" placeholder="Только IP, напр. 114.114.114.114" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>Основные DNS (Nameservers)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.nameserver || []).join('\n')" @blur="updateDnsArray($event, 'nameserver')" rows="3" placeholder="Рекомендуется DoH / DoT" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>Резервные DNS (Fallback)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.fallback || []).join('\n')" @blur="updateDnsArray($event, 'fallback')" rows="3" placeholder="Для зарубежных доменов" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>DNS прямого соединения (Direct Nameservers)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.directNameserver || []).join('\n')" @blur="updateDnsArray($event, 'directNameserver')" rows="2" placeholder="DNS для правил прямого соединения" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>DNS доменов узлов (Proxy Server Nameserver)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.proxyServerNameserver || []).join('\n')" @blur="updateDnsArray($event, 'proxyServerNameserver')" rows="2" placeholder="Для резолвинга доменов прокси-узлов" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>DNS по доменам (Nameserver Policy)</h4></div>
              <textarea class="modern-textarea" :value="formatNameserverPolicy(dnsConfig.nameserverPolicy)" @blur="updateNameserverPolicy" rows="4" placeholder="geosite:cn: https://doh.pub/dns-query" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>GeoIP-фолбэк (Fallback Filter GeoIP)</h4></div>
              <label class="modern-switch">
                <input type="checkbox" v-model="dnsConfig.fallbackFilter.geoip" @change="saveDns" :disabled="!dnsConfig.enable">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info"><h4>Код GeoIP (GeoIP Code)</h4></div>
              <input type="text" class="modern-input" v-model="dnsConfig.fallbackFilter.geoipCode" @blur="saveDns" :disabled="!dnsConfig.enable || !dnsConfig.fallbackFilter.geoip" placeholder="По умолчанию CN" />
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>Фильтр IPCIDR (Fallback Filter IPCIDR)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.fallbackFilter.ipcidr || []).join('\n')" @blur="updateFallbackFilterIpcidr" rows="3" placeholder="напр. 240.0.0.0/4" :disabled="!dnsConfig.enable"></textarea>
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info"><h4>Фильтр доменов (Fallback Filter Domain)</h4></div>
              <textarea class="modern-textarea" :value="(dnsConfig.fallbackFilter.domain || []).join('\n')" @blur="updateFallbackFilterDomain" rows="3" placeholder="Совпавшие домены идут через Fallback" :disabled="!dnsConfig.enable"></textarea>
            </div>

          </div>
        </div>

        <div v-else-if="view === 'network'" class="settings-page">
          <div class="sub-header section-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
            </button>
            <h3>Базовые настройки сети</h3>
            <button class="action-btn accent-btn mini-btn-reset" @click="confirmReset('network')">
              <span class="btn-icon" v-html="ICONS.refresh"></span> Сброс
            </button>
          </div>

          <div class="glass-card setting-group scrollable">
            <div class="setting-item">
              <div class="info">
                <h4>Порт локального прокси (Mixed Port)</h4>
                <p>SOCKS5 и HTTP на этом порту (127.0.0.1). По умолчанию 7890.</p>
              </div>
              <input
                type="number" min="1" max="65535"
                class="modern-input"
                style="text-align: center; width: 110px; font-size: 0.95rem; padding: 10px 12px;"
                v-model.number="netConfig.mixedPort"
                @change="saveNet"
                placeholder="7890"
              />
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Поддержка IPv6</h4>
                <p>Ядро будет резолвить и обрабатывать IPv6. Без поддержки в сети возможны лаги.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="netConfig.ipv6" @change="saveNet">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Доступ из локальной сети (Allow LAN)</h4>
                <p>Другие устройства в сети смогут выходить через этот прокси.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="netConfig.allowLan" @change="saveNet">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Единая задержка (Unified Delay)</h4>
                <p>Убирает затраты на хендшейк — задержка узлов реалистичнее.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="netConfig.unifiedDelay" @change="saveNet">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Конкурентный TCP</h4>
                <p>Соединение сразу со всеми IP, берётся самый быстрый. Заметно ускоряет загрузку.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="netConfig.tcpConcurrent" @change="saveNet">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>TCP Keep Alive (Keep Alive)</h4>
                <p>Меньше обрывов за файрволами, длинные соединения живут дольше.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="netConfig.tcpKeepAlive" @change="saveNet">
                <span class="slider"></span>
              </label>
            </div>

            <Transition name="dropdown">
              <div v-if="netConfig.tcpKeepAlive" class="tcp-keep-alive-sub-items">
                <div class="divider"></div>
                <div class="setting-item">
                  <div class="info">
                    <h4>Интервал отправки (Interval)</h4>
                    <p>В секундах, рекомендуется 15-30s</p>
                  </div>
                  <div class="input-with-unit">
                    <ModernNumberInput 
                      v-model="netConfig.tcpKeepAliveInterval" 
                      :min="1" 
                      :max="3600" 
                      @change="saveNet" 
                    />
                    <span class="unit">s</span>
                  </div>
                </div>
              </div>
            </Transition>

            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info">
                <h4>URL проверки задержки (Delay Test URL)</h4>
                <p>URL проверки доступности. Лучше адреса Google или Cloudflare.</p>
              </div>
              <input 
                type="text" 
                class="modern-input" 
                style="text-align: left; width: 100%; margin-top: 12px; font-size: 0.95rem; padding: 12px 16px;" 
                v-model="netConfig.testUrl" 
                @blur="saveNet" 
                placeholder="http://www.gstatic.com/generate_204" 
              />
            </div>
            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info">
                <h4>Адрес внешнего контроллера (External Controller)</h4>
                <p>Адрес REST API ядра. По умолчанию только 127.0.0.1, менять не стоит.</p>
              </div>
              <input 
                type="text" 
                class="modern-input" 
                style="text-align: left; width: 100%; margin-top: 12px; font-size: 0.95rem; padding: 12px 16px;" 
                v-model="netConfig.externalController" 
                @blur="saveNet" 
                placeholder="127.0.0.1:9090" 
              />
            </div>

            <div class="divider"></div>

            <div class="setting-item col-item">
              <div class="info">
                <h4>Локальный hosts (Hosts)</h4>
                <p>Ручное сопоставление домен → IP. Работает с опцией «Использовать hosts» в DNS.</p>
              </div>
              <div class="hosts-input-container">
                <textarea 
                  class="modern-textarea" 
                  v-model="netConfig.hosts" 
                  @blur="saveNet" 
                  rows="6" 
                  placeholder="'example.com': 127.0.0.1 (формат YAML ключ: значение)"
                  style="margin-top: 10px; font-family: var(--font-mono); font-size: 0.85rem; width: 100%;"
                ></textarea>
                
                <div v-show="hostsError" class="validation-error">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" class="warn-icon" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                  </svg>
                  <span>{{ hostsError }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div v-else-if="view === 'behavior'" class="settings-page">
          <div class="sub-header section-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
            </button>
            <h3>Поведение приложения</h3>
            <button class="action-btn accent-btn mini-btn-reset" @click="confirmReset('behavior')">
              <span class="btn-icon" v-html="ICONS.refresh"></span> Сброс
            </button>
          </div>

          <div class="glass-card setting-group scrollable">
            <div class="setting-item">
              <div class="info">
                <h4>Тихий запуск</h4>
                <p>Запуск сразу в трей, без главного окна.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.silentStart" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Скрывать в трей при закрытии</h4>
                <p>По кнопке закрытия программа продолжит работать в фоне.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.closeToTray" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Автозапуск</h4>
                <p>Запуск Misetanibox при входе в систему.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.startupWithOS" @change="handleStartupWithOSChange">
                <span class="slider"></span>
              </label>
            </div>
            
                        <Transition name="dropdown">
              <div v-if="behavior.startupWithOS" class="delay-retention-sub-items">
                <div class="divider"></div>
                
                <div class="setting-item">
                  <div class="info">
                    <h4>Восстанавливать прокси после запуска</h4>
                    <p>После автозапуска вернуть системный прокси или режим TUN, как до выхода.</p>
                  </div>
                  <label class="modern-switch">
                    <input type="checkbox" v-model="behavior.restoreOnStartup" @change="saveBehavior">
                    <span class="slider"></span>
                  </label>
                </div>

              </div>
            </Transition>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Автопроверка задержки</h4>
                <p>Фоновое обновление задержки узлов с заданным интервалом.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.autoDelayTest" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>

            <Transition name="dropdown">
              <div v-if="behavior.autoDelayTest" class="delay-retention-sub-items">
                <div class="divider"></div>
                <div class="setting-item">
                  <div class="info">
                    <h4>Интервал проверки</h4>
                  </div>
                  <div class="input-with-unit">
                    <ModernNumberInput 
                      v-model="behavior.autoDelayTestInterval" 
                      :min="1"
                      :max="1440"
                      @change="saveBehavior"
                    />
                    <span class="unit">min</span>
                  </div>
                </div>
              </div>
            </Transition>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Цветные значения задержки</h4>
                <p>Задержка узлов зелёным/жёлтым/красным вместо чёрно-белого стиля.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.colorDelay" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Хранение результатов задержки</h4>
                <p>Кэш результатов проверки: очистка по таймеру или долгое хранение.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.delayRetention" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>

            <Transition name="dropdown">
              <div v-if="behavior.delayRetention" class="delay-retention-sub-items">
                <div class="divider"></div>
                <div class="setting-item">
                  <div class="info">
                    <h4>Время хранения</h4>
                  </div>
                  <ModernSelect 
                    v-model="behavior.delayRetentionTime" 
                    :options="[
                      { label: '5 с', value: '5' },
                      { label: '10 с', value: '10' },
                      { label: '30 с', value: '30' },
                      { label: 'Долго', value: 'long' }
                    ]" 
                    @change="saveBehavior" 
                  />
                </div>
              </div>
            </Transition>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Уровень логов ядра</h4>
                <p>Детализация логов ядра. Для разбора проблем включите отладку.</p>
              </div>
              <ModernSelect 
                v-model="behavior.logLevel" 
                :options="logLevelOptions" 
                @change="saveBehavior" 
              />
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Уровень логов приложения</h4>
                <p>Уровень логов самой программы. Применяется сразу, фильтрует страницу логов.</p>
              </div>
              <ModernSelect 
                v-model="behavior.appLogLevel" 
                :options="appLogLevelOptions" 
                @change="saveBehavior" 
              />
            </div>
            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Простой режим (Lite)</h4>
                <p>Упрощённый интерфейс: список серверов и одна кнопка подключения (глобальный режим). Удобно и близко к мобильному виду.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" :checked="globalState.uiMode === 'lite'" @change="onToggleLite">
                <span class="slider"></span>
              </label>
            </div>

            <div class="divider" v-if="isWindows"></div>

            <div class="setting-item" v-if="isWindows">
              <div class="info">
                <h4>Умное ядро (Smart) <span v-if="smartCoreOn" style="color: var(--green-text); font-size: 0.8rem; margin-left: 6px;">установлено</span></h4>
                <p>Заменяет ядро на сборку с ML-выбором лучшего сервера по истории задержек и нагрузке. Скачивается отдельно (~20 МБ). Ядро неподписанное — при первом запуске Windows Defender может его заблокировать; тогда разрешите файл в «Журнале защиты» или добавьте исключение.</p>
                <p style="color: var(--mid-text, #d9a441); font-weight: 600;">⚠ Внимание: Smart-ядро может ломать dialer-proxy (мост/цепочки) вашего провайдера — «умный выбор» уводит трафик мимо мостовых групп, а сборка ядра может не применять префиксы провайдера. Если пользуетесь мостом через промежуточный узел — держите Smart выключенным.</p>
              </div>
              <button
                class="action-btn"
                :class="{ 'accent-btn': !smartCoreOn }"
                :disabled="smartCoreBusy"
                @click="toggleSmartCore"
              >
                {{ smartCoreBusy ? 'Подождите…' : (smartCoreOn ? 'Вернуть обычное' : 'Установить Smart') }}
              </button>
            </div>

            <template v-if="isWindows && smartCoreOn">
              <div class="divider"></div>
              <div class="setting-item">
                <div class="info">
                  <h4>Использовать Смарт для всего</h4>
                  <p>Весь прокси-трафик направляется через Смарт-группу (умный выбор узла) даже в режиме «по правилам». Прямые правила (РФ-сайты напрямую, блок рекламы) сохраняются.</p>
                </div>
                <label class="modern-switch">
                  <input type="checkbox" :checked="smartRouteOn" @change="onToggleSmartRoute">
                  <span class="slider"></span>
                </label>
              </div>
            </template>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Рвать соединения при смене сервера</h4>
                <p>Новый сервер применяется сразу: старые сессии закрываются, чтобы трафик пошёл через выбранный узел без перезапуска туннеля. Выключите, если не хотите обрывать активные загрузки при переключении.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" :checked="closeConnOnSwitch" @change="onToggleCloseConn">
                <span class="slider"></span>
              </label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Скрыть раздел логов</h4>
                <p>Прячет пункт логов в сайдбаре; последние логи хранятся для диагностики.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.hideLogs" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Считать только прокси-трафик</h4>
                <p>График трафика учитывает только прокси-узлы, без прямого (DIRECT) трафика.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.proxyTrafficOnly" @change="saveBehavior">
                <span class="slider"></span>
              </label>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>User-Agent обновления подписки</h4>
                <p>Свой заголовок при загрузке/обновлении подписки; пусто = по умолчанию.</p>
              </div>
              <input 
                type="text" 
                class="modern-input" 
                style="width: 200px; text-align: center;" 
                v-model="behavior.subUA" 
                @blur="saveBehavior" 
                placeholder="UA по умолчанию" 
              />
            </div>
          </div>
        </div>

        <div v-else-if="view === 'about'" class="settings-page">
          <div class="sub-header section-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <svg viewBox="0 0 24 24" width="18" height="18" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"></line><polyline points="12 19 5 12 12 5"></polyline></svg>
            </button>
            <h3>О приложении</h3>
          </div>

          <div class="glass-card setting-group scrollable">
                        <div class="setting-item" style="padding: 20px 0; display: flex; justify-content: space-between; align-items: center;">
              <div class="info" style="display: flex; align-items: center; gap: 18px;">
                <img :src="appLogo" style="width: 52px; height: 52px; border-radius: 12px;" />
                <div>
                  <h4 style="margin: 0; font-weight: 800; font-size: 1.6rem; letter-spacing: -0.01em;">Misetanibox</h4>
                  <a href="javascript:void(0)" @click="openLink('https://t.me/whxteangel')" style="font-size: 0.75rem; color: var(--text-muted); display: block;">Логотип: @whxteangel</a>
                  <a href="javascript:void(0)" @click="openLink('https://github.com/Gleb-pro-admin')" style="font-size: 0.75rem; color: var(--text-muted); display: block;">Конвертер Xray→mihomo: @Gleb-pro-admin</a>
                </div>
              </div>

                                          <div v-if="globalState.appUpdateProgress" class="app-update-progress-container"
                   :class="{ 'clickable-progress': globalState.appUpdateProgress.isDownloaded }"
                   @click="globalState.appUpdateProgress.isDownloaded ? promptInstallApp(globalState.appUpdateProgress) : null">
                <div class="progress-info">
                  <span v-if="globalState.appUpdateProgress.isDownloaded" class="speed" style="color: var(--accent); font-weight: 600;">Новая версия готова — установить</span>
                  <template v-else>
                    <span class="speed">{{ formatSpeed(globalState.appUpdateProgress.speedBps) }}</span>
                    <span class="divider-dot">·</span>
                    <span class="eta">Осталось {{ formatEtaTime(globalState.appUpdateProgress.etaSec) }}</span>
                  </template>
                </div>
                <div class="progress-bar-wrap">
                  <div class="progress-bar-fill" :style="{ width: appUpdatePercent + '%', backgroundColor: globalState.appUpdateProgress.isDownloaded ? 'var(--accent)' : '' }"></div>
                </div>
                <div class="progress-size">
                  <span v-if="globalState.appUpdateProgress.isDownloaded">{{ globalState.appUpdateProgress.version }} — загружено</span>
                  <span v-else>{{ formatBytes(globalState.appUpdateProgress.bytesDone) }} / {{ formatBytes(globalState.appUpdateProgress.totalBytes) }}</span>
                </div>
              </div>
            </div>

            <div class="divider"></div>
            <div class="setting-item">
              <div class="info">
                <h4>Версия приложения</h4>
                <p>{{ globalState.appVersion || 'Получение…' }}</p>
              </div>
              <button class="action-btn accent-btn" @click="handleCheckUpdate" :disabled="globalState.appUpdateChecking">
                {{ globalState.appUpdateChecking ? 'Проверка…' : 'Проверить обновления' }}
              </button>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Автообновление</h4>
                <p>Автопроверка и уведомление о новых версиях.</p>
              </div>
              <label class="modern-switch">
                <input type="checkbox" v-model="behavior.autoUpdate" @change="saveBehavior" />
                <span class="slider"></span>
              </label>
            </div>

            <Transition name="dropdown">
              <div v-if="behavior.autoUpdate" class="auto-update-sub-items">
                <div class="divider"></div>
                <div class="setting-item">
                  <div class="info">
                    <h4>Способ проверки</h4>
                  </div>
                  <ModernSelect 
                    v-model="behavior.updateMethod" 
                    :options="[
                      { label: 'При каждом запуске', value: 'startup' }, 
                      { label: 'По расписанию', value: 'scheduled' }
                    ]" 
                    @change="saveBehavior" 
                  />
                </div>

                <div class="divider"></div>
                <div class="setting-item" :class="{ 'disabled-fade': behavior.updateMethod !== 'scheduled' }">
                  <div class="info">
                    <h4>Интервал проверки</h4>
                  </div>
                  <div class="input-with-unit">
                    <ModernNumberInput 
                      v-model="behavior.updateInterval" 
                      :min="1"
                      :max="365"
                      :disabled="behavior.updateMethod !== 'scheduled'" 
                      @change="saveBehavior"
                    />
                    <span class="unit">д</span>
                  </div>
                </div>
              </div>
            </Transition>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Локальная резервная копия</h4>
                <p>Экспорт подписок, настроек и темы в файл .gocz</p>
              </div>
              <button class="action-btn accent-btn" @click="handleExportBackup">Экспорт копии</button>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Восстановление копии</h4>
                <p>Восстановление из .gocz; подписки сольются умным слиянием</p>
              </div>
              <button class="action-btn accent-btn" @click="openRestoreModal">Восстановление копии</button>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Диагностика каталога данных</h4>
                <p>Каталоги программы/данных, встроенный seed и статус компонентов</p>
              </div>
              <button class="action-btn" @click="openDataDirDiagnosticModal">Показать статус</button>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Диагностика приложения</h4>
                <p>Экспорт путей, ресурсов и статуса служб для разбора проблем</p>
              </div>
              <button class="action-btn accent-btn" @click="handleExportDiagnostics">Экспорт диагностики</button>
            </div>

            <div class="divider"></div>

            <div class="setting-item">
              <div class="info">
                <h4>Репозиторий GitHub</h4>
                <a href="javascript:void(0)" @click="openLink('https://github.com/rakexzp/misetanibox')" class="link-item">https://github.com/rakexzp/misetanibox</a>
              </div>
            </div>
          </div>
        </div>

        <div v-else-if="view === 'uwp'" class="settings-page">

          <div class="sub-header page-sticky-mask">
            <button class="back-btn" @click="view = 'main'">
              <span class="icon back-icon-svg" v-html="ICONS.arrowLeft"></span>
            </button>
            <h3>Управление UWP loopback</h3>
          </div>

          <div class="uwp-toolbar">
            <div class="uwp-search">
              <span class="search-icon" v-html="ICONS.search"></span>
              <input v-model="uwpSearch" placeholder="Поиск по имени или пакету…" />
              <span v-if="uwpSearch" class="clear-icon" @click="uwpSearch = ''" v-html="ICONS.close"></span>
            </div>
            <div class="uwp-batch">
              <button class="batch-btn" @click="toggleAllUwp(true)">Выбрать все</button>
              <button class="batch-btn" @click="toggleAllUwp(false)">Инвертировать</button>
            </div>
          </div>

          <div class="uwp-list-wrapper scrollable">
            <div 
              v-for="app in filteredUwpApps" 
              :key="app.sid" 
              class="uwp-app-item"
              :class="{ 'active': app.isEnabled }"
              @click="app.isEnabled = !app.isEnabled"
            >
              <div class="app-main-content">
                <div class="app-avatar">
                  {{ app.displayName?.[0]?.toUpperCase() || '?' }}
                </div>
                <div class="app-details">
                  <span class="app-name">{{ app.displayName || 'Без имени' }}</span>
                  <span class="app-pkg">{{ app.packageFamilyName }}</span>
                </div>
              </div>

              <div class="app-status-wrapper">
                <div class="uwp-status-tag">
                  {{ app.isEnabled ? 'Исключено' : 'Ограничено' }}
                </div>
              </div>
            </div>
          </div>

          <div class="uwp-footer">
            <button class="apply-btn" :disabled="savingUwp" @click="saveUwpChanges">
              <span v-if="!savingUwp">Применить (нужны права админа)</span>
              <span v-else class="loading-spinner">Сохранение…</span>
            </button>
          </div>
        </div>
      </div>
    </Transition>

    <Transition name="pop">
      <div class="modal-overlay" v-if="showDbModal" @click.self="showDbModal = false">
        <div class="custom-modal-card" @click.stop>
          <div class="modal-header">
            <h3>Ссылка загрузки {{ dbTitles[editingDb.type] }}</h3>
          </div>
          <div class="modal-body">
            <input type="text" class="modal-input" v-model="editingDb.link" style="text-align: left;" @keyup.enter="saveDbLink" />
            <div class="modal-footer">
              <button class="action-btn flex-1" @click="showDbModal = false">Отмена</button>
              <button class="primary-btn accent-btn flex-1" @click="saveDbLink">Сохранить</button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
        <Transition name="pop">
      <div v-if="showResetConfirm" class="modal-overlay" @click="showResetConfirm = false">
        <div class="custom-modal-card" @click.stop>
          <div class="modal-header">
            <h3 class="danger-text">Подтверждение сброса</h3>
          </div>
          <div class="modal-body">
            <p class="global-modal-msg">Сбросить <strong>{{ resetModuleName }}</strong> к значениям по умолчанию? Действие необратимо, конфигурация будет перезагружена.</p>
            <div class="modal-footer">
              <button class="action-btn flex-1" @click="showResetConfirm = false">Отмена</button>
              <button class="primary-btn accent-btn red-text-btn flex-1" @click="handleReset">Сбросить</button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
    
    <Transition name="pop">
      <div v-if="showCoreUpdateConfirm" class="modal-overlay" @click="cancelCoreUpdateConfirm">
        <div class="custom-modal-card" @click.stop>
          <div class="modal-header">
            <h3>Доступна новая версия</h3>
          </div>
          <div class="modal-body">
            <p class="global-modal-msg">
              Найдена новая версия ядра Mihomo <strong>{{ globalState.componentUpdate.coreUpdateInfo.remote }}</strong>, текущая — <strong>{{ globalState.componentUpdate.coreUpdateInfo.local }}</strong>.<br/><br/>
              Обновление ядра кратко разорвёт прокси. Обновить сейчас?
            </p>
            <div class="modal-footer">
              <button class="action-btn flex-1" @click="cancelCoreUpdateConfirm">Отмена</button>
              <button class="primary-btn accent-btn flex-1" @click="executeCoreUpdate">Обновить сейчас</button>
            </div>
          </div>
        </div>
      </div>
    </Transition>

        <Transition name="pop">
      <div v-if="showRestoreModal" class="modal-overlay" @click.self="showRestoreModal = false">
        <div class="custom-modal-card" @click.stop>
          <div class="modal-header">
            <h3>Восстановление данных</h3>
          </div>
          <div class="modal-body">
            <p class="global-modal-msg">Выберите файл копии и режим восстановления:</p>
            
            <div class="restore-actions" style="width: 100%; display: flex; flex-direction: column; gap: 4px;">
              <button class="action-btn w-full-btn hover-accent" @click="handleSelectFile" :class="{'active-border': selectedPath}" style="width: 100%; box-sizing: border-box;">
                <span class="btn-icon" v-html="ICONS.folder" style="margin-right: 4px;"></span>
                <span class="truncate" style="flex: 1; text-align: center;">
                  {{ selectedPath ? 'Выбран: ' + selectedPath.split('\\').pop() : 'Выбрать файл (.gocz)' }}
                </span>
              </button>
              
              <div class="divider-text" style="margin: 12px 0">Режим восстановления</div>
              
              <div class="mode-selector-group" style="width: 100%;">
                <ModernSelect 
                  v-model="restoreMode" 
                  :options="[
                    { 
                      label: 'Всё (настройки и подписки)', 
                      value: 'all',
                      description: 'Полный возврат настроек, подписок и основной конфигурации; текущие данные будут перезаписаны.'
                    },
                    { 
                      label: 'Подписки (заменить список)', 
                      value: 'subs',
                      description: 'Список подписок из копии заменит текущий; лишние подписки удалятся.'
                    },
                    { 
                      label: 'Подписки (слить со списком)', 
                      value: 'subs-merge',
                      description: 'Текущие подписки останутся, новые из копии добавятся; совпавшие ID перезапишутся.'
                    },
                    { 
                      label: 'Настройки (с темой/логами)', 
                      value: 'settings',
                      description: 'Только поведение, DNS, сеть и тема; подписки не затрагиваются.'
                    }
                  ]"
                />
              </div>
            </div>

            <div class="modal-footer">
              <button class="action-btn flex-1" @click="showRestoreModal = false">Отмена</button>
              <button class="primary-btn accent-btn flex-1" :disabled="!selectedPath" @click="confirmRestore">Восстановить</button>
            </div>
          </div>
        </div>
      </div>
    </Transition>

    <Transition name="pop">
      <div v-if="showDataDirDiagnosticModal" class="modal-overlay" @click.self="showDataDirDiagnosticModal = false">
        <div class="custom-modal-card" @click.stop style="max-width: 500px;">
          <div class="modal-header">
            <h3>Диагностика каталога данных</h3>
          </div>
          <div class="modal-body">
            <div v-if="dataDirInfo" class="diagnostic-results" style="max-height: 400px; overflow-y: auto;">
              <div class="setting-item" style="padding: 6px 0; align-items: flex-start;">
                <div class="info">
                  <h4>Каталог программы</h4>
                  <p class="link-text" style="word-break: break-all;">{{ dataDirInfo.appDir }}</p>
                </div>
              </div>
              <div class="setting-item" style="padding: 6px 0; align-items: flex-start;">
                <div class="info">
                  <h4>Каталог данных</h4>
                  <p class="link-text" style="word-break: break-all;">{{ dataDirInfo.dataDir }}</p>
                </div>
              </div>
              <div class="setting-item" style="padding: 6px 0; align-items: flex-start;">
                <div class="info">
                  <h4>Встроенный seed-каталог</h4>
                  <p class="link-text" style="word-break: break-all;">{{ dataDirInfo.seedCoreBinDir }}</p>
                  <p v-if="dataDirInfo.seedManifestExists" style="color: var(--green-text); font-size: 0.8rem; margin-top: 4px;">seed-манифест: есть</p>
                  <p v-else style="color: var(--red-text); font-size: 0.8rem; margin-top: 4px;">seed-манифест: отсутствует</p>
                </div>
              </div>
              <div class="setting-item" style="padding: 6px 0; align-items: flex-start;">
                <div class="info">
                  <h4>Каталог компонентов</h4>
                  <p class="link-text" style="word-break: break-all;">{{ dataDirInfo.coreBinDir }}</p>
                  <p v-if="dataDirInfo.layoutOK" style="color: var(--green-text); font-size: 0.8rem; margin-top: 4px;">Структура: в норме</p>
                  <p v-else style="color: var(--accent); font-size: 0.8rem; margin-top: 4px;">Структура: нарушена, будет исправлена при следующем запуске</p>
                </div>
              </div>
              <div class="setting-item" style="padding: 6px 0; align-items: flex-start;">
                <div class="info">
                  <h4>Статус компонентов</h4>
                  <p :style="{ color: dataDirInfo.coreReady ? 'var(--green-text)' : 'var(--red-text)', fontWeight: 600 }">
                    Mihomo: {{ dataDirInfo.coreReady ? 'готов' : (dataDirInfo.coreExists ? 'повреждён' : 'отсутствует') }}
                  </p>
                  <p :style="{ color: dataDirInfo.wintunReady ? 'var(--green-text)' : 'var(--accent)', fontWeight: 600 }">
                    Wintun: {{ dataDirInfo.wintunReady ? 'готов' : (dataDirInfo.wintunExists ? 'повреждён' : 'отсутствует') }}
                  </p>
                </div>
              </div>
              <div class="setting-item" style="padding: 6px 0; align-items: flex-start; border-bottom: none;">
                <div class="info">
                  <h4>Старый каталог</h4>
                  <p class="link-text" style="word-break: break-all;">{{ dataDirInfo.legacyDataDir || 'нет' }}</p>
                  <p v-if="dataDirInfo.legacyExists" style="color: var(--accent); font-size: 0.8rem; margin-top: 4px;">Ещё есть, будет перенесён при запуске</p>
                  <p v-else style="color: var(--green-text); font-size: 0.8rem; margin-top: 4px;">Нет или уже удалён</p>
                </div>
              </div>
            </div>
            
            <div class="modal-footer" style="margin-top: 16px;">
              <button class="action-btn flex-1" @click="openDataDirDiagnosticModal">Проверить снова</button>
              <button class="action-btn flex-1" @click="handleExportDiagnostics">Экспорт диагностики</button>
              <button class="primary-btn accent-btn flex-1" @click="showDataDirDiagnosticModal = false">Готово</button>
            </div>
          </div>
        </div>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, onUnmounted, computed, watch } from 'vue';
import * as API from '../../wailsjs/go/main/App';
import { BrowserOpenURL, EventsOn } from '../../wailsjs/runtime/runtime';

import { showAlert, showConfirm, globalState, setUiMode } from '../store';

// Разделы, применимые только к Windows (wintun, фоновая служба, Smart-ядро, GPU-тумблер).
// На Linux/macOS прячем: они либо не нужны, либо собираются только под Windows.
const isWindows = computed(() => globalState.platform === 'windows' || globalState.platform === '');
const isLinux = computed(() => globalState.platform === 'linux');

import { getSavedColors, saveColors, type CustomColors } from '../utils/colors';
import { economyEnabled, setEconomy } from '../utils/perf';
import { formatBytes, formatSpeed, formatEtaTime, formatRelativeTime } from '../utils/format';
import { ICONS } from '../utils/icons';
import appLogo from '../assets/logo.ico';
import UpdateTaskPanel from './UpdateTaskPanel.vue';

import ModernSelect from './ModernSelect.vue';
import ModernNumberInput from './ModernNumberInput.vue';

const openLink = (url: string) => {
  BrowserOpenURL(url);
};

// --- редактор цветов (покраска букв/фона/акцента) ---
const themeColorDefaults = (): CustomColors =>
  globalState.theme === 'dark'
    ? { accent: '#E8E8E8', text: '#E8E8E8', bg: '#111111' }
    : { accent: '#1A1A1A', text: '#1A1A1A', bg: '#F0F0F0' };
const colors = reactive<CustomColors>(getSavedColors() || themeColorDefaults());
const COLOR_PRESETS = [
  { name: 'Reaper', accent: '#C0392B', text: '#ECECEC', bg: '#0E0E10' },
  { name: 'Nord', accent: '#88C0D0', text: '#ECEFF4', bg: '#2E3440' },
  { name: 'Матрица', accent: '#39FF14', text: '#C8FFC8', bg: '#0A0F0A' },
  { name: 'Сепия', accent: '#8B5E34', text: '#3B2F2F', bg: '#F4ECD8' },
  { name: 'Океан', accent: '#2193B0', text: '#EAF6FA', bg: '#0B1E26' },
];
const onColorInput = () => saveColors({ ...colors });
const applyColorPreset = (p: { accent: string; text: string; bg: string }) => {
  colors.accent = p.accent; colors.text = p.text; colors.bg = p.bg;
  saveColors({ ...colors });
};
const resetColors = () => {
  saveColors(null);
  Object.assign(colors, themeColorDefaults());
};

// экономный режим анимаций (меньше нагрузки на GPU)
const economy = ref(economyEnabled());
const toggleEconomy = () => { economy.value = !economy.value; setEconomy(economy.value); };

// разгрузка GPU интерфейса (отключение композитинга webview) — применяется при перезапуске
const gpuSaver = ref(false);
(async () => { try { gpuSaver.value = await (API.GetGpuSaver as any)(); } catch { /* ignore */ } })();
const toggleGpuSaver = async () => {
  gpuSaver.value = !gpuSaver.value;
  try {
    await (API.SetGpuSaver as any)(gpuSaver.value);
    await showAlert('Настройка сохранена. Перезапустите приложение, чтобы она применилась.', 'Нужен перезапуск');
  } catch (e) { await showAlert('Не удалось сохранить: ' + e, 'Ошибка'); }
};

const showResetConfirm = ref(false);
const resetModule = ref('');
const resetModuleName = ref('');
const showCoreUpdateConfirm = ref(false);
const showRestoreModal = ref(false);
const selectedPath = ref("");
const restoreMode = ref("all");

const modules: Record<string, string> = {
  'network': 'Базовые настройки сети',
  'dns': 'Настройки DNS-серверов',
  'tun': 'Виртуальный адаптер',
  'behavior': 'Поведение приложения'
};

const confirmReset = (mod: string) => {
  resetModule.value = mod;
  resetModuleName.value = modules[mod];
  showResetConfirm.value = true;
};

const handleReset = async () => {
  try {
    await API.ResetComponentSettings(resetModule.value);
    showResetConfirm.value = false;
    
    if (resetModule.value === 'network') {
        const netConf = await (API.GetNetworkConfig as any)();
        if (netConf) netConfig.value = netConf;
    }
    if (resetModule.value === 'dns') {
        const dnsConf = await (API.GetDNSConfig as any)();
        if (dnsConf) dnsConfig.value = dnsConf;
    }
    if (resetModule.value === 'tun') {
        const tunConf = await API.GetTunConfig();
        if (tunConf) tunConfig.value = tunConf;
    }
    if (resetModule.value === 'behavior') {
        const bh = await API.GetAppBehavior();
        if (bh) behavior.value = bh;
    }
    showAlert(`${resetModuleName.value} — сброшено к значениям по умолчанию`, "Успешно");
  } catch (e) {
    console.error("重置失败:", e);
    showAlert("Сброс не удался: " + e, "Ошибка");
  }
};

const props = defineProps({
  initialView: {
    type: String,
    default: 'main'
  }
});

const view = ref(props.initialView as 'main' | 'uwp' | 'tun' | 'dns' | 'network' | 'behavior' | 'update' | 'about');
watch(() => props.initialView, (newVal) => { view.value = newVal as any; });

watch(view, async (v) => {
  setTimeout(() => {
    document.querySelector('.view-scroller')?.scrollTo({ top: 0, behavior: 'auto' });
  }, 250);

  if (v === 'update') {
    await refreshComponentInfo();
  }
});

const coreVersion = ref('Чтение…');
const wintunVersion = ref('Чтение…');
const isInstalling = ref(false);

const dbList = [
  { key: 'geoip', title: 'GeoIP', behaviorKey: 'geoIpLink' },
  { key: 'geosite', title: 'GeoSite', behaviorKey: 'geoSiteLink' },
  { key: 'mmdb', title: 'MMDB', behaviorKey: 'mmdbLink' },
  { key: 'asn', title: 'ASN', behaviorKey: 'asnLink' },
];
const dbTitles: Record<string, string> = { geoip: 'GeoIP', geosite: 'GeoSite', mmdb: 'MMDB', asn: 'ASN' };

// LWIP убран: ядро (metacubex mihomo) его больше не поддерживает — выбор ломал TUN.
const stackOptions = [
  { label: 'gVisor', value: 'gvisor' },
  { label: 'Mixed', value: 'mixed' },
  { label: 'System', value: 'system' },
];
const STACK_HINTS: Record<string, string> = {
  gvisor: 'Полностью в userspace. Самый совместимый и стабильный, но медленнее. Выбирай при странных обрывах.',
  mixed: 'TCP через ядро (быстро), UDP через gVisor (надёжно). Золотая середина — рекомендуется для скорости.',
  system: 'Всё через сетевой стек ОС. Максимум скорости и меньше нагрузка на CPU, но совместимость ниже (проверь игры/звонки).',
};
const stackHint = computed(() => STACK_HINTS[tunConfig.value?.stack] || STACK_HINTS.gvisor);

const enhancedModeOptions = [
  { label: 'Fake-IP', value: 'fake-ip' },
  { label: 'Redir-Host', value: 'redir-host' },
  { label: 'Normal', value: 'normal' }
];

const logLevelOptions = [
  { label: 'Отладка', value: 'debug' },
  { label: 'Инфо', value: 'info' },
  { label: 'Предупреждения', value: 'warn' },
  { label: 'Ошибки', value: 'error' },
  { label: 'Тихо', value: 'silent' }
];

const appLogLevelOptions = [
  { label: 'Отладка', value: 'debug' },
  { label: 'Инфо', value: 'info' },
  { label: 'Предупреждения', value: 'warn' },
  { label: 'Ошибки', value: 'error' }
];

const showDbModal = ref(false);
const editingDb = ref({ type: '', link: '' });
const dbFileInfo = ref<Record<string, any>>({});

const isUpdatingDb = (key: string) => globalState.componentUpdate.tasks[key]?.status === 'running';

const refreshRuntimeAssets = async () => {
  const status = await (API as any).GetRuntimeAssetStatus();
  globalState.assetStatus = status;

  const core = status?.assets?.core;
  const wintun = status?.assets?.wintun;

  coreVersion.value = core?.ready
    ? (core.version || 'Установлено, версия неизвестна')
    : (core?.error || core?.hint || 'Не установлено');

  wintunVersion.value = wintun?.ready
    ? (wintun.version || 'Установлено, версия неизвестна')
    : (wintun?.error || wintun?.hint || 'Не установлено');

  tunStatus.value = {
    ...tunStatus.value,
    hasWintun: !!wintun?.ready,
    wintunError: wintun?.ready ? '' : (wintun?.error || wintun?.hint || 'wintun недоступен'),
    wintun,
  };

  dbFileInfo.value = {
    geoip: status?.assets?.geoip || {},
    geosite: status?.assets?.geosite || {},
    mmdb: status?.assets?.mmdb || {},
    asn: status?.assets?.asn || {},
  };
};

const refreshComponentFileInfo = refreshRuntimeAssets;
const refreshComponentInfo = refreshRuntimeAssets;

const isUpdatingAnyDb = computed(() => {
  return ["geoip", "geosite", "mmdb", "asn"].some(isUpdatingDb);
});

const formatUpdateError = (err: any) => {
  let msg = String(err || '');
  msg = msg.replace(/https:\/\/release-assets\.githubusercontent\.com\/\S+/g, 'ссылка ресурса GitHub Release');
  
  msg = msg.replace(/https:\/\/github\.com\/\S+\/releases\/download\/\S+/g, (match) => {
    try {
      const url = new URL(match);
      url.search = '';
      return url.toString();
    } catch (e) {
      return 'ссылка GitHub Release';
    }
  });

  msg = msg.replace(/([?&](sp|sv|se|sr|sig|skoid|sktid|skt|ske|sks|skv)=[^\\s]+)/g, '');

  if (msg.length > 360) {
    msg = msg.slice(0, 360) + '...';
  }
  return msg;
};

const handleCheckUpdate = async () => {
  if (globalState.appUpdateChecking) return;
  try {
    await (API as any).CheckAndDownloadAppUpdateAsync();
  } catch (e) {
    await showAlert("Проверка обновлений не удалась: " + e, "Ошибка", true);
  }
};

const promptInstallApp = async (progress: any) => {
  const version = progress.version || "";
  const fullPath = progress.path || "";

  const ok = await showConfirm(
      `Misetanibox ${version} загружен.\n\n` +
      `Закрыть программу и запустить установщик?\n\n` +
      `После установки временный пакет будет удалён.`,
      "Новая версия загружена",
      false
  );
  
  if (ok) {
      if (!fullPath) {
        await showAlert("Путь к установщику пуст, скачайте обновление заново.", "Ошибка", true);
        return;
      }
      try {
        await (API as any).ApplyAppUpdate(fullPath);
      } catch (e: any) {
        await showAlert(String(e?.message || e || "Неизвестная ошибка"), "Не удалось запустить установщик", true);
      }
  }
};

const handleExportBackup = async () => {
  try {
    const res = await (API as any).ExportBackup();
    if (res === "SUCCESS") {
      await showAlert("Резервная копия экспортирована", "Уведомление");
    }
  } catch (e) {
    await showAlert("Экспорт не удался: " + String(e), "Ошибка");
  }
};

const openRestoreModal = () => {
  selectedPath.value = "";
  restoreMode.value = "all";
  showRestoreModal.value = true;
};

const handleSelectFile = async () => {
  try {
    const path = await (API as any).SelectBackupFile();
    if (path) {
      selectedPath.value = path;
    }
  } catch (e) {
    console.error("选择文件取消或失败", e);
  }
};

const confirmRestore = async () => {
  if (!selectedPath.value) return;

  const warnings: Record<string, string> = {
    all: 'Полное восстановление заменит настройки, подписки, рабочую конфигурацию и тему. Ядро будет кратко остановлено.',
    subs: 'Список подписок из копии заменит текущий; лишние подписки будут удалены.',
    'subs-merge': 'Текущие подписки останутся, подписки из копии добавятся; совпавшие ID могут быть перезаписаны.',
    settings: 'Восстановятся только настройки и тема; подписки не изменятся.'
  };

  const confirmMsg = warnings[restoreMode.value] || 'Выполнить восстановление данных?';

  const ok = await showConfirm(
    confirmMsg + "\n\nПосле восстановления часть настроек применится сразу.",
    "Подтверждение восстановления",
    true
  );
  if (!ok) return;

  try {
    const res = await (API as any).ExecuteRestore(selectedPath.value, restoreMode.value);
    if (res === "SUCCESS") {
      showRestoreModal.value = false;
      await showAlert("Данные восстановлены! Настройки применены.", "Успешно");
    }
  } catch (e) {
    await showAlert("Восстановление не удалось: " + String(e), "Ошибка");
  }
};

const handleUpdateCore = async () => {
  if (globalState.componentUpdate.checkingCoreUpdate || globalState.componentUpdate.updatingCore) return;
  (API as any).CheckCoreUpdateAsync().catch(() => {});
};

const cancelCoreUpdateConfirm = () => {
  showCoreUpdateConfirm.value = false;
  globalState.componentUpdate.pendingCoreUpdate = false;
  globalState.componentUpdate.checkingCoreUpdate = false;
};

const executeCoreUpdate = () => {
  showCoreUpdateConfirm.value = false;
  if (globalState.componentUpdate.updatingCore) return;
  (API as any).UpdateCoreComponentAsync().catch(() => {});
};

const tunStatus = ref<Record<string, any>>({ hasWintun: false, isAdmin: false, wintunError: '' });

const tunConfig = ref({
  stack: 'gvisor', device: '', autoRoute: true, autoDetect: true,
  dnsHijack: ['any:53'], strictRoute: true, mtu: 1500, gso: false
});

const dnsConfig = ref<any>({
  enable: true, 
  listen: '0.0.0.0:1053',
  ipv6: false, 
  preferH3: false,
  enhancedMode: 'fake-ip', 
  respectRules: false,
  fakeIpRange: '198.18.0.1/16',
  fakeIpFilter: ['*.lan', '*.localdomain'],
  useSystemHosts: true,
  useHosts: true,
  defaultNameserver: ['223.5.5.5', '114.114.114.114'],
  nameserver: ['https://doh.pub/dns-query'],
  fallback: ['https://doh.dns.sb/dns-query'],
  directNameserver: ['https://dns.alidns.com/dns-query'],
  proxyServerNameserver: ['https://doh.pub/dns-query'],
  nameserverPolicy: { 'geosite:cn': 'https://doh.pub/dns-query' },
  fallbackFilter: {
      geoip: true,
      geoipCode: 'CN',
      ipcidr: ['240.0.0.0/4', '0.0.0.0/32'],
      domain: ['+.google.com', '+.facebook.com', '+.twitter.com']
  }
});

const netConfig = ref({
  mixedPort: 7890,
  ipv6: false,
  allowLan: false,
  externalController: '127.0.0.1:9090',
  unifiedDelay: true,
  tcpConcurrent: true,
  tcpKeepAlive: true,
  tcpKeepAliveInterval: 15,
  testUrl: 'http://www.gstatic.com/generate_204',
  hosts: ''
});

const hostsError = ref('');

const validateHosts = (val: string) => {
  if (!val || val.trim() === '') {
    hostsError.value = ''; // 为空是合法的（代表不配置）
    return true;
  }

  const lines = val.split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trim();
    
    if (line === '' || line.startsWith('#')) continue;

    if (!/^[^:]+:\s*.+$/.test(line)) {
      hostsError.value = `Строка ${i + 1}: неверный формат. Нужно "домен: IP" (двоеточие латинское, значение обязательно)`;
      return false;
    }
  }
  
  hostsError.value = ''; // 校验通过，清空错误
  return true;
};

watch(() => netConfig.value.hosts, (newVal) => {
  validateHosts(newVal || '');
});

const behavior = ref<any>({
  silentStart: false,
  closeToTray: true,
    startupWithOS: false,
    restoreOnStartup: false,
  colorDelay: false,
  delayRetention: false,          // 👇 移到了这里
  delayRetentionTime: 'long',     // 👇 移到了这里
  proxyTrafficOnly: false,
  logLevel: 'info',
  hideLogs: false,
  subUA: '',
  activeConfig: '',
  activeMode: '',
  geoIpLink: '',
  geoSiteLink: '',
  mmdbLink: '',
  asnLink: '',
  autoUpdate: true,
  updateMethod: 'startup',
  updateInterval: 3,
});

const uwpApps = ref<any[]>([]);
const uwpSearch = ref('');
const savingUwp = ref(false);

const enterUwpManager = async () => {
  view.value = 'uwp';
  try {
    uwpApps.value = await (API as any).GetUwpApps();
  } catch (e) {
    showAlert('Не удалось получить список UWP: ' + e, 'Ошибка');
  }
};

const filteredUwpApps = computed(() => {
  const q = uwpSearch.value.toLowerCase();
  return uwpApps.value.filter(app => 
    (app.displayName || '').toLowerCase().includes(q) || 
    (app.packageFamilyName || '').toLowerCase().includes(q)
  );
});

const toggleAllUwp = (val: boolean) => {
  if (val) {
    uwpApps.value.forEach(app => app.isEnabled = true);
  } else {
    uwpApps.value.forEach(app => app.isEnabled = !app.isEnabled);
  }
};

const saveUwpChanges = async () => {
  savingUwp.value = true;
  try {
    const sids = uwpApps.value.filter(a => a.isEnabled).map(a => a.sid);
    await (API as any).SaveUwpExemptions(sids);
    await showAlert('Исключения обновлены!', 'Готово');
  } catch (e) {
    await showAlert('Сохранение не удалось: ' + e, 'Ошибка');
  } finally {
    savingUwp.value = false;
  }
};

const loadData = async () => {
  try {
    const [_, status, tunConf, dnsConf, netConf, behaviorConf] = await Promise.all([
      refreshRuntimeAssets(),
      API.CheckTunEnv(),
      API.GetTunConfig(),
      (API.GetDNSConfig as any)(),
      (API.GetNetworkConfig as any)(),
      (API.GetAppBehavior as any)(),
    ]);

    tunStatus.value = status;
    if (tunConf) tunConfig.value = tunConf;
    if (dnsConf) dnsConfig.value = dnsConf;
    if (netConf) netConfig.value = netConf;
    if (behaviorConf) {
      behavior.value = behaviorConf;
      refreshHelperStatus();
    }
  } catch (e) {
    console.error('加载配置失败', e);
  }
};

const unsubs: (() => void)[] = [];

onMounted(() => {
  loadData();
  refreshSmartCore();

  unsubs.push(EventsOn("core-version-updated", (payload: any) => {
    coreVersion.value = payload?.version || coreVersion.value;
    void showAlert(`Ядро обновлено, текущая версия: ${coreVersion.value}`, "Обновление успешно");
  }));

  unsubs.push(EventsOn("core-update-none", () => {
    void showAlert("Уже последняя версия, обновление не нужно.", "Проверка обновлений");
  }));

  unsubs.push(EventsOn("wintun-version-updated", (payload: any) => {
    wintunVersion.value = payload?.version || wintunVersion.value;
  }));

  watch(() => globalState.componentUpdate.pendingCoreUpdate, (newVal) => {
    if (newVal) {
      showCoreUpdateConfirm.value = true;
    }
  });
  unsubs.push(EventsOn("app-update-busy", () => {
    globalState.appUpdateChecking = false;
    void showAlert("Обновление уже выполняется, попробуйте позже.", "Уведомление");
  }));

  ['geoip', 'geosite', 'mmdb', 'asn'].forEach(key => {
    unsubs.push(EventsOn(`geo-update-${key}-success`, refreshComponentInfo));
  });

  unsubs.push(EventsOn("driver-install-start", () => {
    isInstalling.value = true;
  }));
  unsubs.push(EventsOn("driver-install-success", () => {
    isInstalling.value = false;
    refreshComponentInfo(); // To update wintun version immediately if needed, or rely on wintun-version-updated
  }));
  unsubs.push(EventsOn("driver-install-error", () => {
    isInstalling.value = false;
  }));
});

onUnmounted(() => {
  unsubs.forEach(unsub => unsub && unsub());
});

const handleTunToggle = async (e: Event) => {
  const target = e.target as HTMLInputElement;
  const newState = target.checked;

  if (newState && !tunStatus.value.hasWintun) {
    e.preventDefault();
    target.checked = false;
    await showAlert('Не удалось включить режим TUN:\nСначала нажмите «Установить драйвер» ниже, чтобы скачать и настроить wintun.dll.', 'Нет компонента');
    return;
  }
  
  globalState.tun = newState;
  
  try {
    await API.ToggleTunMode(newState);
  } catch (err) {
    globalState.tun = !newState;
    target.checked = !newState;
    await showAlert("Операция TUN не удалась: " + err, 'Ошибка');
  }
};

const installDriver = async (force: boolean = true) => {
  if (isInstalling.value) return;
  const ok = await showConfirm(
    "Во время установки сеть кратко прервётся. При активном режиме TUN ядро перезапустится автоматически.",
    "Переустановить драйвер Wintun?",
    false
  );
  if (!ok) return;

  (API as any).InstallTunDriverAsync(force).catch(() => {});
};

watch(() => behavior.value.updateInterval, async (newVal) => {
  if (newVal !== undefined && newVal <= 0) {
    behavior.value.updateInterval = 1;
    
    if (behavior.value.autoUpdate && behavior.value.updateMethod === 'scheduled') {
      await showAlert("Интервал проверки — не меньше 1 дня.", "Подсказка");
    }
    
    saveBehavior();
  }
});

const saveTun = async () => {
  try { await API.SaveTunConfig(tunConfig.value); } catch (e) { console.error('保存失败', e); }
};

const appUpdatePercent = computed(() => {
  const p = globalState.appUpdateProgress;
  if (!p || !p.totalBytes) return 0;
  return Math.min(100, Math.floor((p.bytesDone / p.totalBytes) * 100));
});

const updateTunDnsHijack = (e: Event) => {
  const val = (e.target as HTMLInputElement).value;
  tunConfig.value.dnsHijack = val.split(',').map(s => s.trim()).filter(s => s);
  saveTun();
};

const saveDns = async () => {
  try { await (API.SaveDNSConfig as any)(dnsConfig.value); } catch (e) { console.error('DNS 保存失败', e); }
};

const saveNet = async () => {
  try {
    await (API.SaveNetworkConfig as any)(netConfig.value);
  } catch (e) {
    console.error('网络配置保存失败', e);
  }
};

const saveBehavior = async () => {
  try {
    await API.SaveAppBehavior(behavior.value);
    if (behavior.value.appLogLevel) {
      globalState.appLogLevel = behavior.value.appLogLevel;
    }
  } catch (e) {
    console.error('应用行为保存失败', e);
  }
};

const onToggleLite = (e: Event) => {
  const on = (e.target as HTMLInputElement).checked;
  setUiMode(on ? 'lite' : 'full');
};

const smartCoreOn = ref(false);
const smartCoreBusy = ref(false);
const smartRouteOn = ref(false);

const closeConnOnSwitch = ref(true);

const refreshSmartCore = async () => {
  try { smartCoreOn.value = await (API as any).IsSmartCore(); } catch { smartCoreOn.value = false; }
  try { smartRouteOn.value = await (API as any).GetSmartRoute(); } catch { smartRouteOn.value = false; }
  try { closeConnOnSwitch.value = await (API as any).GetCloseConnOnSwitch(); } catch { closeConnOnSwitch.value = true; }
};

const onToggleCloseConn = async (e: Event) => {
  const on = (e.target as HTMLInputElement).checked;
  try { await (API as any).SetCloseConnOnSwitch(on); closeConnOnSwitch.value = on; }
  catch (err) { await showAlert('Не удалось переключить: ' + err, 'Ошибка', true); }
};

const onToggleSmartRoute = async (e: Event) => {
  const on = (e.target as HTMLInputElement).checked;
  try {
    await (API as any).SetSmartRoute(on);
    smartRouteOn.value = on;
  } catch (err) {
    await showAlert('Не удалось переключить: ' + err, 'Ошибка', true);
  }
};

const toggleSmartCore = async () => {
  if (smartCoreBusy.value) return;
  if (!smartCoreOn.value) {
    const ok = await showConfirm(
      'Скачать и установить умное ядро? Оно неподписанное — если Windows Defender его заблокирует, разрешите файл в защитнике. Текущее ядро сохранится для отката.',
      'Умное ядро (Smart)'
    );
    if (!ok) return;
  }
  smartCoreBusy.value = true;
  try {
    if (smartCoreOn.value) {
      await (API as any).RemoveSmartCore();
    } else {
      await (API as any).InstallSmartCore();
    }
    await refreshSmartCore();
    await showAlert(
      smartCoreOn.value
        ? 'Умное ядро установлено. В списке серверов появится «Смарт».'
        : 'Возвращено обычное ядро.',
      'Готово'
    );
  } catch (e) {
    await showAlert('Не удалось: ' + e, 'Ошибка', true);
  } finally {
    smartCoreBusy.value = false;
  }
};

const handleStartupWithOSChange = async () => {
  if (!behavior.value.startupWithOS) {
    behavior.value.restoreOnStartup = false;
  }
  saveBehavior();
};

const helperStatus = ref<any>({ installed: false, running: false, reachable: false });
const helperLoading = ref(false);

const refreshHelperStatus = async () => {
  helperLoading.value = true;
  try {
    helperStatus.value = await (API as any).GetHelperServiceStatus();
  } catch (e) {
    console.error('获取 Helper 服务状态失败', e);
  } finally {
    helperLoading.value = false;
  }
};

const installHelper = async () => {
  helperLoading.value = true;
  try {
    await (API as any).InstallHelperService();
    await refreshHelperStatus();
    showAlert('Фоновая служба установлена', 'Готово');
  } catch (e) {
    showAlert('Установка службы не удалась: ' + e, 'Ошибка', true);
  } finally {
    helperLoading.value = false;
  }
};

const restartHelper = async () => {
  helperLoading.value = true;
  try {
    await (API as any).RestartHelperService();
    await refreshHelperStatus();
    showAlert('Фоновая служба перезапущена', 'Готово');
  } catch (e) {
    showAlert('Перезапуск службы не удался: ' + e, 'Ошибка', true);
  } finally {
    helperLoading.value = false;
  }
};

const showDataDirDiagnosticModal = ref(false);
const dataDirInfo = ref<any>(null);

const openDataDirDiagnosticModal = async () => {
  try {
    dataDirInfo.value = await API.GetDataDirInfo();
    showDataDirDiagnosticModal.value = true;
  } catch (error) {
    showAlert('Не удалось получить данные каталога: ' + error);
  }
};

const handleExportDiagnostics = async () => {
  try {
    await API.ExportDiagnostics();
    showAlert('Диагностика экспортирована');
  } catch (error) {
    if (error && String(error).indexOf('User cancelled') === -1) {
      showAlert('Экспорт диагностики не удался: ' + error);
    }
  }
};

const openDbEditModal = (type: string, currentLink: string) => {
  editingDb.value = { type, link: currentLink };
  showDbModal.value = true;
};

const saveDbLink = async () => {
  const t = editingDb.value.type;
  const match = dbList.find(d => d.key === t);
  if (match) {
    behavior.value[match.behaviorKey] = editingDb.value.link;
  }
  showDbModal.value = false;
  await saveBehavior();
};

const handleUpdateDb = async (key: string) => {
  try {
    await (API as any).UpdateGeoDatabaseAsync(key);
  } catch (e) {
    void showAlert(`${key}: не удалось запустить обновление: ${formatUpdateError(e)}`, 'Ошибка', true);
  }
};

const handleUpdateAllDbs = async () => {
  try {
    await API.UpdateAllGeoDatabasesAsync();
  } catch (e) {
    void showAlert(`Не удалось запустить обновление: ${formatUpdateError(e)}`, 'Ошибка', true);
  }
};

const updateDnsArray = (e: Event, key: string) => {
  const val = (e.target as HTMLTextAreaElement).value;
  dnsConfig.value[key] = val.split('\n').map(s => s.trim()).filter(s => s);
  saveDns();
};

const updateFallbackFilterIpcidr = (e: Event) => {
    const val = (e.target as HTMLTextAreaElement).value;
    dnsConfig.value.fallbackFilter.ipcidr = val.split('\n').map(s => s.trim()).filter(s => s);
    saveDns();
};

const updateFallbackFilterDomain = (e: Event) => {
    const val = (e.target as HTMLTextAreaElement).value;
    dnsConfig.value.fallbackFilter.domain = val.split('\n').map(s => s.trim()).filter(s => s);
    saveDns();
};

const formatNameserverPolicy = (policy: Record<string, string>) => {
  if (!policy) return '';
  return Object.entries(policy).map(([k, v]) => `${k}: ${v}`).join('\n');
};

const updateNameserverPolicy = (e: Event) => {
  const val = (e.target as HTMLTextAreaElement).value;
  const policy: Record<string, string> = {};

  val.split('\n').forEach(line => {
    line = line.trim();
    if (!line) return;

    let idx = line.indexOf(': ');
    if (idx === -1) idx = line.lastIndexOf(':');

    if (idx > 0) {
      const k = line.substring(0, idx).trim();
      const v = line.substring(idx + 1).trim();
      if (k && v) policy[k] = v;
    }
  });

  dnsConfig.value.nameserverPolicy = policy;
  saveDns();
};
</script>

<style scoped>
.settings-container { 
  display: flex; 
  flex-direction: column; 
  min-height: 100%;
  overflow: visible;
  position: relative; 
}

.clickable-progress {
  cursor: pointer;
  padding: 10px 14px;
  border-radius: 12px;
  transition: background 0.2s;
  margin-right: -14px;
  margin-top: -10px;
  margin-bottom: -10px;
}
.clickable-progress:hover {
  background: var(--surface-hover);
}

.settings-view-wrapper {
  display: flex;
  flex-direction: column;
  width: 100%;
}
.settings-page { 
  display: flex; 
  flex-direction: column; 
  flex: 1; 
  min-height: 100%;
  overflow: visible;
}
.setting-group { padding: 20px 24px; margin-bottom: 12px; }
.setting-group.scrollable { 
  padding-bottom: 20px; 
  overflow: visible;
  max-height: none;
}

h3 { margin: 0 0 8px 0; color: var(--text-main); font-size: 1.25rem; padding-bottom: 4px; }
h4 { margin: 0 0 6px 0; color: var(--text-main); font-size: 1rem;}
p { 
  margin: 0; 
  font-size: 0.85rem; 
  color: var(--text-sub); 
  max-width: 100%;
  line-height: 1.5;
}

.info { flex: 1; padding-right: 24px; min-width: 0; }

.setting-item { display: flex; justify-content: space-between; align-items: center; padding: 14px 0; }
.col-item { flex-direction: column; align-items: stretch; gap: 10px; padding: 16px 0; }
.setting-item.clickable { cursor: pointer; padding: 16px; border-radius: 12px; margin: 0 -16px; transition: 0.2s; }
.setting-item.clickable:hover { background: var(--surface-hover); }

.arrow { color: var(--text-sub); font-size: 1.2rem; }
.divider { height: 1px; background: var(--glass-border); opacity: 0.5; margin: 0; }

.btn-group {
  display: flex;
  gap: 8px;
}

.modern-input, .modern-textarea { 
  background: var(--surface-hover); 
  border: none; 
  color: var(--text-main); 
  padding: 10px 14px; 
  border-radius: 8px; 
  outline: none; 
  font-size: 0.9rem;
}

.modern-select {
  background-color: var(--surface-hover);
  border: 1px solid transparent;
  color: var(--text-main);
  padding: 8px 32px 8px 12px; 
  border-radius: 8px;
  outline: none;
  cursor: pointer;
  font-size: 0.9rem;
  font-family: inherit;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  appearance: none;
  -webkit-appearance: none;
  background-image: url("data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='24' height='24' viewBox='0 0 24 24' fill='none' stroke='%23777777' stroke-width='2' stroke-linecap='round' stroke-linejoin='round'%3E%3Cpolyline points='6 9 12 15 18 9'%3E%3C/polyline%3E%3C/svg%3E");
  background-repeat: no-repeat;
  background-position: right 10px center;
  background-size: 16px;
}

.modern-select:hover:not(:disabled) {
  background-color: var(--surface-panel);
}

.modern-select:focus {
  border: 1px solid var(--text-sub);
  background-color: var(--surface);
}

.modern-select:disabled { 
  opacity: 0.5; 
  cursor: not-allowed; 
}

.modern-input { text-align: right; }
.modern-textarea { resize: vertical; font-family: monospace; font-size: 0.85rem; line-height: 1.5; text-align: left; }
.modern-input:disabled, .modern-textarea:disabled { opacity: 0.5; cursor: not-allowed; }
.num-input { width: 80px; }

.modern-switch { position: relative; display: inline-block; width: 44px; height: 24px; }
.modern-switch input { opacity: 0; width: 0; height: 0; }

.slider { position: absolute; cursor: pointer; top: 0; left: 0; right: 0; bottom: 0; background-color: var(--surface-hover); transition: .3s; border-radius: 24px; box-shadow: inset 0 1px 3px rgba(0,0,0,0.1); }
.slider:before { position: absolute; content: ""; height: 18px; width: 18px; left: 3px; bottom: 3px; background-color: white; transition: .3s; border-radius: 50%; box-shadow: 0 1px 3px rgba(0,0,0,0.3);}
input:disabled + .slider { opacity: 0.5; cursor: not-allowed; }
input:checked + .slider { background-color: var(--accent); }
input:checked + .slider:before { transform: translateX(20px); background-color: var(--accent-fg); }

.slide-in { animation: slideIn 0.2s ease forwards; }
@keyframes slideIn { from { opacity: 0; transform: translateX(10px); } to { opacity: 1; transform: translateX(0); } }

.sub-header { 
  display: flex; 
  align-items: center; 
  gap: 16px; 
  margin-bottom: 12px; 
  padding: 0 0 12px 0;
  background: transparent;
}
.sub-header.page-sticky-mask {
  --sticky-mask-bleed: 4px;
}
.sub-header h3 { margin: 0; border: none; padding: 0; }
.back-btn { background: var(--surface); border: none; color: var(--text-main); width: 36px; height: 36px; border-radius: 50%; display: flex; align-items: center; justify-content: center; cursor: pointer; transition: 0.2s; }
.back-btn:hover { background: var(--surface-hover); }

.sub-item {
  padding-left: 20px !important;
  border-left: 2px solid var(--surface-hover);
  margin-left: 8px;
  margin-top: -10px;
  margin-bottom: 10px;
}

.sub-label { font-size: 0.9rem !important; font-weight: 500 !important; }
.disabled-fade { opacity: 0.5; pointer-events: none; }
.input-with-unit { display: flex; align-items: center; gap: 8px; }
.unit { font-size: 0.85rem; color: var(--text-sub); font-family: var(--font-mono); font-weight: 500; }
.status-msg { margin-top: 4px; font-weight: 500; }
.green-text { color: var(--text-main); font-weight: 600; }
.red-text { color: var(--text-muted); }

.uwp-toolbar {
  display: flex;
  align-items: center;
  gap: 16px;
  margin-bottom: 20px;
}

.uwp-search {
  flex: 1;
  display: flex;
  align-items: center;
  background: var(--surface);
  border: 1px solid var(--surface-hover);
  border-radius: 10px;
  padding: 0 12px;
  height: 40px;
  transition: all 0.2s;
}

.uwp-search:focus-within {
  border-color: var(--accent);
  background: var(--surface-panel);
}

.uwp-search input {
  flex: 1;
  border: none;
  background: transparent;
  color: var(--text-main);
  outline: none;
  margin-left: 8px;
  font-size: 0.9rem;
}

.search-icon, .clear-icon {
  display: flex;
  align-items: center;
  color: var(--text-sub);
}

.clear-icon {
  cursor: pointer;
  padding: 4px;
}

.uwp-batch {
  display: flex;
  gap: 8px;
}

.batch-btn {
  background: var(--surface-hover);
  color: var(--text-main);
  border: none;
  padding: 8px 16px;
  border-radius: 8px;
  font-size: 0.85rem;
  font-weight: 600;
  cursor: pointer;
  transition: 0.2s;
}
.batch-btn:hover { background: var(--surface-panel); }

.uwp-list-wrapper {
  display: flex;
  flex-direction: column;
  gap: 10px; 
  flex: 1;
  padding-right: 4px;
}

.uwp-app-item {
  background: var(--surface);
  border-radius: 12px; 
  padding: 12px 16px;
  display: flex;
  justify-content: space-between;
  align-items: center;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
  border: 1px solid transparent;
}

.uwp-app-item:hover {
  background: var(--surface-hover);
}

.uwp-app-item.active {
  background: var(--accent);
  box-shadow: 0 4px 15px rgba(0, 0, 0, 0.1);
}

.app-main-content {
  display: flex;
  align-items: center;
  gap: 16px;
  overflow: hidden;
  flex: 1;
}

.app-avatar {
  width: 42px;
  height: 42px;
  background: var(--surface-panel);
  border-radius: 10px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 800;
  font-size: 1.3rem;
  color: var(--text-sub);
  flex-shrink: 0;
}
.uwp-app-item.active .app-avatar {
  background: rgba(255, 255, 255, 0.15);
  color: var(--accent-fg);
}

.app-details {
  display: flex;
  flex-direction: column;
  gap: 2px;
  overflow: hidden;
}

.app-name {
  font-size: 0.95rem;
  font-weight: 700;
  color: var(--text-main);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.uwp-app-item.active .app-name { color: var(--accent-fg); }

.app-pkg {
  font-size: 0.75rem;
  color: var(--text-sub);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  opacity: 0.7;
}
.uwp-app-item.active .app-pkg { color: var(--accent-fg); opacity: 0.8; }

.uwp-status-tag {
  font-size: 0.7rem; 
  letter-spacing: 0; 
  font-weight: 600;
  padding: 3px 10px;
  border-radius: 4px; 
  text-transform: uppercase;
  transition: all 0.2s;
  background: var(--surface-panel);
  color: var(--text-main);
}

.uwp-app-item.active .uwp-status-tag {
  background: var(--accent-fg) !important;
  color: var(--accent) !important;
  opacity: 0.8;
}

.uwp-footer {
  margin-top: 20px;
  padding-top: 10px;
}

.apply-btn {
  width: 100%;
  padding: 14px;
  background: var(--accent);
  color: var(--accent-fg);
  border: none;
  border-radius: 12px;
  font-weight: 700;
  cursor: pointer;
  transition: all 0.2s;
  box-shadow: 0 4px 10px rgba(0, 0, 0, 0.1);
}

.apply-btn:hover:not(:disabled) {
  filter: brightness(1.1);
  transform: translateY(-1px);
}

.apply-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.loading-spinner {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.back-btn .icon svg {
  width: 18px;
  height: 18px;
  display: block;
}

.back-icon-svg :deep(svg) {
  width: 18px;
  height: 18px;
}

.link-text { font-family: monospace; font-size: 0.8rem; color: var(--text-muted); margin-top: 4px; }

.modal-footer .action-btn, 
.modal-footer .primary-btn {
  height: 42px;
  display: flex;
  align-items: center;
  justify-content: center;
}

.slide-fade-enter-active,
.slide-fade-leave-active {
  transition: all 0.25s cubic-bezier(0.4, 0, 0.2, 1);
  width: 100%;
  height: 100%;
}

.slide-fade-enter-from {
  opacity: 0;
  transform: translateX(12px); 
}

.slide-fade-leave-to {
  opacity: 0;
  transform: translateX(-12px); 
}
.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.sub-header.section-header h3 {
  flex: 1;
  margin-left: 12px;
}

.mini-btn-reset {
  height: 36px !important;
  padding: 0 14px !important;
  font-size: 0.85rem !important;
  border-radius: 8px !important;
}

.mini-btn-reset :deep(.btn-icon) svg {
  width: 16px;
  height: 16px;
}

.w-full-btn { width: 100%; justify-content: center; }
.divider-text { 
  display: flex; align-items: center; text-align: center; color: var(--text-sub); font-size: 0.75rem; 
  font-weight: 600; text-transform: uppercase; letter-spacing: 0.05em; margin: 15px 0;
}
.divider-text::before, .divider-text::after { content: ''; flex: 1; border-bottom: 1px solid var(--surface-hover); }
.divider-text::before { margin-right: 10px; }
.divider-text::after { margin-left: 10px; }

.restore-actions { display: flex; flex-direction: column; }
.active-border { border: 1px solid var(--accent) !important; }
.w-full { width: 100%; }

.dropdown-enter-active,
.dropdown-leave-active {
  transition: all 0.5s cubic-bezier(0.4, 0, 0.2, 1);
  max-height: 250px;
  overflow: hidden;
}

.dropdown-enter-from,
.dropdown-leave-to {
  opacity: 0;
  max-height: 0;
  transform: translateY(-8px);
}

.link-item {
  color: var(--accent);
  font-size: 0.85rem;
  text-decoration: none;
  transition: opacity 0.2s;
  cursor: pointer;
}
.link-item:hover {
  opacity: 0.8;
  text-decoration: underline;
}
.hosts-input-container {
  display: flex;
  flex-direction: column;
  gap: 8px;
  width: 100%;
}

.validation-error {
  display: flex;
  align-items: center;
  gap: 6px;
  color: #f59e0b; /* 橙色警告色 */
  font-size: 0.85rem;
  font-weight: 500;
  animation: fadeIn 0.3s ease;
  margin-top: 4px;
}

.warn-icon {
  width: 16px;
  height: 16px;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(-4px); }
  to { opacity: 1; transform: translateY(0); }
}
.app-update-progress-container {
  flex: 1;
  max-width: 240px;
  display: flex;
  flex-direction: column;
  gap: 6px;
  background: var(--surface-hover);
  padding: 10px 14px;
  border-radius: 8px;
}

.progress-info {
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 0.8rem;
  font-weight: 600;
  color: var(--text-main);
}

.divider-dot {
  color: var(--text-muted);
  font-weight: bold;
}

.progress-bar-wrap {
  width: 100%;
  height: 6px;
  background: var(--surface-panel);
  border-radius: 3px;
  overflow: hidden;
}

.progress-bar-fill {
  height: 100%;
  background: var(--accent);
  transition: width 0.3s ease;
}

.progress-size {
  font-size: 0.75rem;
  color: var(--text-muted);
  text-align: right;
  font-variant-numeric: tabular-nums;
}

/* редактор цветов */
.color-row { display: flex; align-items: center; gap: 14px; padding: 12px 0; }
.color-swatch {
  width: 40px; height: 40px; border-radius: 10px; flex: none; cursor: pointer;
  border: 1px solid var(--surface-hover); position: relative; overflow: hidden;
  box-shadow: inset 0 0 0 2px var(--surface-panel);
}
.color-swatch input[type="color"] {
  position: absolute; inset: -4px; width: calc(100% + 8px); height: calc(100% + 8px);
  border: none; padding: 0; opacity: 0; cursor: pointer;
}
.color-presets { display: flex; flex-wrap: wrap; gap: 8px; padding: 10px 0 4px; }
.color-preset {
  padding: 7px 14px; border-radius: 8px; font-size: 0.8rem; font-weight: 600;
  border: 1.5px solid; cursor: pointer; transition: transform 0.12s;
}
.color-preset:hover { transform: translateY(-1px); }
</style>