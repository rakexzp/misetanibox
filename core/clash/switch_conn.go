package clash

import "goclashz/core/utils"

// closeConnOnSwitchKey — рвать ли активные соединения при смене сервера.
// mihomo меняет узел селектора только для НОВЫХ коннектов; уже открытые TCP-сессии
// висят на старом сервере до закрытия. Без разрыва «сервер меняется только после
// рестарта туннеля». По умолчанию включено (как в Clash Verge).
const closeConnOnSwitchKey = "close_conn_on_switch"

func CloseConnOnSwitchEnabled() bool {
	v, err := utils.LoadSetting(closeConnOnSwitchKey, true)
	if err != nil || v == nil {
		return true
	}
	return *v
}

func SetCloseConnOnSwitch(on bool) error {
	return utils.SaveSetting(closeConnOnSwitchKey, &on)
}
