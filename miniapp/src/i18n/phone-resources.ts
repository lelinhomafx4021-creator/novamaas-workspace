/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export const phoneResources = {
  en: {
    'wechat.editProfile': 'Edit profile',
    'wechat.cancelEdit': 'Cancel editing',
    'wechat.displayProfile': 'Avatar and nickname (optional)',
    'wechat.displayProfileHint':
      'For display only. You can skip these and edit them later in your profile.',
    'wechat.changeAvatar': 'Change avatar',
    'wechat.nicknameLabel': 'WeChat nickname (optional)',
    'wechat.nicknamePlaceholder': 'Select your WeChat nickname',
    'wechat.useWeChatNickname': 'Use WeChat nickname',
    'wechat.nicknameAuthorizationHint':
      'Tap the button, then select “Use WeChat nickname” above the WeChat keyboard. You can skip this step.',
    'wechat.platformRequiredTitle': 'Bind your phone on the platform first',
    'wechat.platformRequiredHint':
      'No platform account has verified this phone number. Verify and bind it on the platform, then return to sign in.',
    'wechat.platformRequiredSteps':
      'If you do not have a platform account, create one there and bind your phone number first.',
    'wechat.connectionHint':
      'Your phone matches an existing platform account. Confirm the connection below.',
    'wechat.currentIdentity': 'WeChat account',
    'wechat.currentWeChat': 'Current WeChat account',
    'wechat.avatarOptional': 'Tap to choose an avatar (optional)',
    'wechat.platformAccount': 'Platform account',
    'wechat.pendingConnection': 'Connection awaiting confirmation',
    'wechat.phoneMatched': 'Verified phone matched',
    'wechat.confirmEffect':
      'After confirmation, this WeChat account can sign in to the platform account shown above.',
    'wechat.additionalVerification':
      'This account requires additional verification. Enter its password and authenticator code if enabled.',
    'wechat.confirmAndSignIn': 'Confirm and sign in',
    'wechat.cancelConnection': 'Cancel connection',
    'auth.error.appIdMismatch':
      'The mini program and server use different AppIDs. Ask an administrator to check the WeChat configuration.',
    'phone.login': 'SMS sign-in',
    'phone.description':
      'Sign in with a phone number already verified on your account.',
    'phone.number': 'Phone number (+86)',
    'phone.code': 'SMS verification code',
    'phone.send': 'Send SMS code',
    'phone.resend': 'Resend in {{seconds}}s',
    'phone.rateLimited': 'Too many SMS requests. Try again later.',
    'phone.unavailable': 'SMS is unavailable. Use another sign-in method.',
    'phone.invalidNumber': 'Enter a valid mainland China phone number.',
    'phone.invalidCode':
      'Verification failed. Check the code or request a new one.',
    'wechat.title': 'WeChat account connection',
    'wechat.bound': 'WeChat connected',
    'wechat.notBound':
      'WeChat is not connected. Use WeChat phone sign-in to connect your account.',
    'wechat.noProfile': 'Nickname and avatar not provided',
    'wechat.boundAt': 'Connected at {{time}}',
    'wechat.profileHint':
      'Nickname and avatar are optional and selected by you. They do not change your account identity.',
    'wechat.chooseAvatar': 'Choose avatar',
    'wechat.nickname': 'WeChat nickname (optional)',
    'wechat.save': 'Save profile',
    'wechat.profileError':
      'Could not save the profile. Use an image smaller than 2 MB and try again.',
    'wechat.unlink': 'Disconnect WeChat',
    'wechat.unlinkHint':
      'Verify with your password and authenticator (if enabled), or your verified phone. All devices will be signed out.',
    'wechat.verifyUnlink': 'Verify and disconnect',
    'wechat.verifyPhone': 'Send code to verified phone',
    'wechat.unlinked': 'WeChat disconnected. Sign in again.',
    'wechat.lastCredential':
      'Add another sign-in method before disconnecting WeChat.',
    'wechat.unlinkError':
      'Could not disconnect WeChat. Verify your account and try again.',
    'wechat.confirm': 'Confirm WeChat connection',
    'wechat.confirmHint':
      'Your verified phone matches account {{account}}. Connect this WeChat identity to that account?',
  },
  zh: {
    'wechat.editProfile': '编辑微信资料',
    'wechat.cancelEdit': '取消编辑',
    'wechat.displayProfile': '头像与昵称（可选）',
    'wechat.displayProfileHint':
      '仅用于展示，可以跳过；绑定后也可在个人中心修改。',
    'wechat.changeAvatar': '更换头像',
    'wechat.nicknameLabel': '微信昵称（可选）',
    'wechat.nicknamePlaceholder': '请选择微信昵称',
    'wechat.useWeChatNickname': '使用微信昵称',
    'wechat.nicknameAuthorizationHint':
      '点击按钮后，在微信键盘上方选择“使用微信昵称”。此步骤可以跳过。',
    'wechat.platformRequiredTitle': '请先在平台绑定手机号',
    'wechat.platformRequiredHint':
      '平台未找到已验证绑定此手机号的账号。请先前往平台完成手机号验证与绑定，再返回小程序登录。',
    'wechat.platformRequiredSteps':
      '如果尚无平台账号，请先在平台创建账号并绑定手机号。',
    'wechat.connectionHint':
      '已通过手机号找到平台账号，请确认下方两个账号的绑定关系。',
    'wechat.currentIdentity': '微信账号',
    'wechat.currentWeChat': '当前微信账号',
    'wechat.avatarOptional': '点击选择头像（可选）',
    'wechat.platformAccount': '平台账号',
    'wechat.pendingConnection': '待确认绑定关系',
    'wechat.phoneMatched': '已匹配验证手机号',
    'wechat.confirmEffect': '确认后，当前微信账号可用于登录上方的平台账号。',
    'wechat.additionalVerification':
      '此账号需要额外验证，请输入该平台账号的密码及已开启的认证器验证码。',
    'wechat.confirmAndSignIn': '确认绑定并登录',
    'wechat.cancelConnection': '取消绑定',
    'auth.error.appIdMismatch':
      '小程序与后端的 AppID 不一致，请管理员核对微信配置后重试。',
    'phone.login': '短信验证码登录',
    'phone.description': '使用账号已验证的手机号登录。',
    'phone.number': '手机号（+86）',
    'phone.code': '短信验证码',
    'phone.send': '发送短信验证码',
    'phone.resend': '{{seconds}} 秒后重发',
    'phone.rateLimited': '短信请求过于频繁，请稍后重试。',
    'phone.unavailable': '短信暂不可用，请使用其他登录方式。',
    'phone.invalidNumber': '请输入有效的中国大陆手机号。',
    'phone.invalidCode': '验证失败，请检查验证码或重新获取。',
    'wechat.title': '微信账号绑定',
    'wechat.bound': '已绑定微信',
    'wechat.notBound': '尚未绑定微信，可通过微信手机号登录关联账号。',
    'wechat.noProfile': '尚未提供昵称和头像',
    'wechat.boundAt': '绑定时间：{{time}}',
    'wechat.profileHint': '昵称和头像由你自愿选择，不影响平台账号身份。',
    'wechat.chooseAvatar': '选择头像',
    'wechat.nickname': '微信昵称（可选）',
    'wechat.save': '保存资料',
    'wechat.profileError': '资料保存失败，请使用小于 2 MB 的图片后重试。',
    'wechat.unlink': '解绑微信',
    'wechat.unlinkHint':
      '请验证密码及已开启的认证器，或验证已绑定手机号。解绑后所有设备会退出登录。',
    'wechat.verifyUnlink': '验证并解绑',
    'wechat.verifyPhone': '发送验证码至已绑定手机号',
    'wechat.unlinked': '微信已解绑，请重新登录。',
    'wechat.lastCredential': '请先添加其他可用登录方式，再解绑微信。',
    'wechat.unlinkError': '微信解绑失败，请验证账号身份后重试。',
    'wechat.confirm': '确认绑定微信',
    'wechat.confirmHint':
      '你的已验证手机号匹配账号 {{account}}。确认将当前微信身份绑定到该账号。',
  },
  'zh-TW': {
    'wechat.editProfile': '編輯微信資料',
    'wechat.cancelEdit': '取消編輯',
    'wechat.displayProfile': '頭像與暱稱（選填）',
    'wechat.displayProfileHint':
      '僅用於顯示，可以略過；綁定後也可在個人中心修改。',
    'wechat.changeAvatar': '更換頭像',
    'wechat.nicknameLabel': '微信暱稱（選填）',
    'wechat.nicknamePlaceholder': '請選擇微信暱稱',
    'wechat.useWeChatNickname': '使用微信暱稱',
    'wechat.nicknameAuthorizationHint':
      '點擊按鈕後，在微信鍵盤上方選擇「使用微信暱稱」。此步驟可以略過。',
    'wechat.platformRequiredTitle': '請先在平台綁定手機號碼',
    'wechat.platformRequiredHint':
      '平台找不到已驗證綁定此手機號碼的帳號。請先至平台完成驗證與綁定，再返回小程式登入。',
    'wechat.platformRequiredSteps':
      '若尚無平台帳號，請先在平台建立帳號並綁定手機號碼。',
    'wechat.connectionHint':
      '已透過手機號碼找到平台帳號，請確認下方兩個帳號的綁定關係。',
    'wechat.currentIdentity': '微信帳號',
    'wechat.currentWeChat': '目前微信帳號',
    'wechat.avatarOptional': '點擊選擇頭像（選填）',
    'wechat.platformAccount': '平台帳號',
    'wechat.pendingConnection': '待確認綁定關係',
    'wechat.phoneMatched': '已符合驗證手機號碼',
    'wechat.confirmEffect': '確認後，目前微信帳號可用於登入上方的平台帳號。',
    'wechat.additionalVerification':
      '此帳號需要額外驗證，請輸入該平台帳號的密碼及已啟用的認證器驗證碼。',
    'wechat.confirmAndSignIn': '確認綁定並登入',
    'wechat.cancelConnection': '取消綁定',
    'auth.error.appIdMismatch':
      '小程式與後端的 AppID 不一致，請管理員核對微信設定後重試。',
    'phone.login': '簡訊驗證碼登入',
    'phone.description': '使用帳號已驗證的手機號碼登入。',
    'phone.number': '手機號碼（+86）',
    'phone.code': '簡訊驗證碼',
    'phone.send': '傳送簡訊驗證碼',
    'phone.resend': '{{seconds}} 秒後重傳',
    'phone.rateLimited': '簡訊請求過於頻繁，請稍後再試。',
    'phone.unavailable': '簡訊暫不可用，請使用其他登入方式。',
    'phone.invalidNumber': '請輸入有效的中國大陸手機號碼。',
    'phone.invalidCode': '驗證失敗，請檢查驗證碼或重新取得。',
    'wechat.title': '微信帳號綁定',
    'wechat.bound': '已綁定微信',
    'wechat.notBound': '尚未綁定微信，可透過微信手機號碼登入關聯帳號。',
    'wechat.noProfile': '尚未提供暱稱和頭像',
    'wechat.boundAt': '綁定時間：{{time}}',
    'wechat.profileHint': '暱稱和頭像由你自願選擇，不影響平台帳號身分。',
    'wechat.chooseAvatar': '選擇頭像',
    'wechat.nickname': '微信暱稱（選填）',
    'wechat.save': '儲存資料',
    'wechat.profileError': '資料儲存失敗，請使用小於 2 MB 的圖片後重試。',
    'wechat.unlink': '解除微信綁定',
    'wechat.unlinkHint':
      '請驗證密碼及已開啟的認證器，或驗證已綁定手機號碼。解除後所有裝置會登出。',
    'wechat.verifyUnlink': '驗證並解除綁定',
    'wechat.verifyPhone': '傳送驗證碼至已綁定手機號碼',
    'wechat.unlinked': '微信已解除綁定，請重新登入。',
    'wechat.lastCredential': '請先新增其他可用登入方式，再解除微信綁定。',
    'wechat.unlinkError': '微信解除綁定失敗，請驗證帳號身分後再試。',
    'wechat.confirm': '確認綁定微信',
    'wechat.confirmHint':
      '你的已驗證手機號碼符合帳號 {{account}}。確認將目前微信身分綁定至該帳號。',
  },
  fr: {
    'wechat.editProfile': 'Modifier le profil',
    'wechat.cancelEdit': 'Annuler les modifications',
    'wechat.displayProfile': 'Avatar et pseudo (facultatifs)',
    'wechat.displayProfileHint':
      'Pour l’affichage uniquement. Vous pouvez les ignorer et les modifier plus tard dans votre profil.',
    'wechat.changeAvatar': 'Changer d’avatar',
    'wechat.nicknameLabel': 'Pseudo WeChat (facultatif)',
    'wechat.nicknamePlaceholder': 'Sélectionnez votre pseudo WeChat',
    'wechat.useWeChatNickname': 'Utiliser le pseudo WeChat',
    'wechat.nicknameAuthorizationHint':
      'Appuyez sur le bouton, puis choisissez « Utiliser le pseudo WeChat » au-dessus du clavier WeChat. Cette étape est facultative.',
    'wechat.platformRequiredTitle': 'Liez votre numéro sur la plateforme',
    'wechat.platformRequiredHint':
      'Aucun compte de la plateforme n’a vérifié ce numéro. Vérifiez-le et liez-le sur la plateforme, puis revenez vous connecter.',
    'wechat.platformRequiredSteps':
      'Sans compte sur la plateforme, créez-en un et liez d’abord votre numéro.',
    'wechat.connectionHint':
      'Votre numéro correspond à un compte existant. Confirmez la liaison ci-dessous.',
    'wechat.currentIdentity': 'Compte WeChat',
    'wechat.currentWeChat': 'Compte WeChat actuel',
    'wechat.avatarOptional': 'Choisir un avatar (facultatif)',
    'wechat.platformAccount': 'Compte plateforme',
    'wechat.pendingConnection': 'Liaison à confirmer',
    'wechat.phoneMatched': 'Numéro vérifié correspondant',
    'wechat.confirmEffect':
      'Après confirmation, ce compte WeChat pourra se connecter au compte de la plateforme ci-dessus.',
    'wechat.additionalVerification':
      'Ce compte exige une vérification supplémentaire. Saisissez son mot de passe et le code d’authentification, si activé.',
    'wechat.confirmAndSignIn': 'Confirmer et se connecter',
    'wechat.cancelConnection': 'Annuler la liaison',
    'auth.error.appIdMismatch':
      'Les AppID du mini-programme et du serveur diffèrent. Demandez à un administrateur de vérifier la configuration WeChat.',
    'phone.login': 'Connexion par SMS',
    'phone.description':
      'Connectez-vous avec un numéro déjà vérifié sur votre compte.',
    'phone.number': 'Numéro de téléphone (+86)',
    'phone.code': 'Code de vérification SMS',
    'phone.send': 'Envoyer le code SMS',
    'phone.resend': 'Renvoyer dans {{seconds}} s',
    'phone.rateLimited': 'Trop de demandes SMS. Réessayez plus tard.',
    'phone.unavailable': 'SMS indisponible. Utilisez une autre méthode.',
    'phone.invalidNumber': 'Saisissez un numéro valide de Chine continentale.',
    'phone.invalidCode':
      'Vérification échouée. Vérifiez le code ou demandez-en un nouveau.',
    'wechat.title': 'Liaison du compte WeChat',
    'wechat.bound': 'WeChat lié',
    'wechat.notBound':
      'WeChat non lié. Connectez-vous avec votre numéro WeChat pour le lier.',
    'wechat.noProfile': 'Pseudo et avatar non fournis',
    'wechat.boundAt': 'Lié le {{time}}',
    'wechat.profileHint':
      'Pseudo et avatar sont facultatifs et choisis par vous. Ils ne modifient pas votre identité.',
    'wechat.chooseAvatar': 'Choisir un avatar',
    'wechat.nickname': 'Pseudo WeChat (facultatif)',
    'wechat.save': 'Enregistrer le profil',
    'wechat.profileError':
      'Échec de sauvegarde. Utilisez une image de moins de 2 Mo.',
    'wechat.unlink': 'Dissocier WeChat',
    'wechat.unlinkHint':
      'Vérifiez votre mot de passe et votre authentificateur, ou votre numéro vérifié. Tous les appareils seront déconnectés.',
    'wechat.verifyUnlink': 'Vérifier et dissocier',
    'wechat.verifyPhone': 'Envoyer au numéro vérifié',
    'wechat.unlinked': 'WeChat dissocié. Reconnectez-vous.',
    'wechat.lastCredential':
      'Ajoutez une autre méthode de connexion avant de dissocier WeChat.',
    'wechat.unlinkError':
      'Dissociation échouée. Vérifiez votre compte et réessayez.',
    'wechat.confirm': 'Confirmer la liaison WeChat',
    'wechat.confirmHint':
      'Votre numéro vérifié correspond au compte {{account}}. Liez cette identité WeChat à ce compte.',
  },
  ja: {
    'wechat.editProfile': 'プロフィールを編集',
    'wechat.cancelEdit': '編集をキャンセル',
    'wechat.displayProfile': 'アバターとニックネーム（任意）',
    'wechat.displayProfileHint':
      '表示用です。省略して、連携後にプロフィールで変更できます。',
    'wechat.changeAvatar': 'アバターを変更',
    'wechat.nicknameLabel': 'WeChat ニックネーム（任意）',
    'wechat.nicknamePlaceholder': 'WeChat ニックネームを選択',
    'wechat.useWeChatNickname': 'WeChat ニックネームを使用',
    'wechat.nicknameAuthorizationHint':
      'ボタンをタップし、WeChat キーボードの上にある「WeChat ニックネームを使用」を選択してください。この手順はスキップできます。',
    'wechat.platformRequiredTitle': '先にプラットフォームで電話番号を連携',
    'wechat.platformRequiredHint':
      'この電話番号を確認済みのアカウントが見つかりません。プラットフォームで番号を確認・連携してから再度ログインしてください。',
    'wechat.platformRequiredSteps':
      'プラットフォームのアカウントがない場合は、先に作成して電話番号を連携してください。',
    'wechat.connectionHint':
      '電話番号に一致するアカウントが見つかりました。以下の連携を確認してください。',
    'wechat.currentIdentity': 'WeChatアカウント',
    'wechat.currentWeChat': '現在のWeChat',
    'wechat.avatarOptional': 'タップしてアバターを選択（任意）',
    'wechat.platformAccount': 'プラットフォームのアカウント',
    'wechat.pendingConnection': '連携確認待ち',
    'wechat.phoneMatched': '確認済み電話番号が一致',
    'wechat.confirmEffect':
      '確認後、このWeChatで上記のプラットフォームアカウントにログインできます。',
    'wechat.additionalVerification':
      'このアカウントは追加確認が必要です。パスワードと、有効な場合は認証器のコードを入力してください。',
    'wechat.confirmAndSignIn': '連携を確認してログイン',
    'wechat.cancelConnection': '連携をキャンセル',
    'auth.error.appIdMismatch':
      'ミニプログラムとサーバーの AppID が一致しません。管理者に WeChat 設定の確認を依頼してください。',
    'phone.login': 'SMSでログイン',
    'phone.description': 'アカウントの確認済み電話番号でログインします。',
    'phone.number': '電話番号（+86）',
    'phone.code': 'SMS確認コード',
    'phone.send': 'SMSコードを送信',
    'phone.resend': '{{seconds}}秒後に再送',
    'phone.rateLimited': 'SMSの要求が多すぎます。後で再試行してください。',
    'phone.unavailable':
      'SMSを利用できません。別のログイン方法を使用してください。',
    'phone.invalidNumber': '有効な中国本土の電話番号を入力してください。',
    'phone.invalidCode':
      '確認できませんでした。コードを確認するか再送してください。',
    'wechat.title': 'WeChatアカウント連携',
    'wechat.bound': 'WeChat連携済み',
    'wechat.notBound':
      'WeChat未連携です。WeChat電話番号でログインして連携できます。',
    'wechat.noProfile': 'ニックネームとアバター未設定',
    'wechat.boundAt': '連携日時：{{time}}',
    'wechat.profileHint':
      'ニックネームとアバターは任意です。アカウントの識別情報は変わりません。',
    'wechat.chooseAvatar': 'アバターを選択',
    'wechat.nickname': 'WeChatニックネーム（任意）',
    'wechat.save': 'プロフィールを保存',
    'wechat.profileError':
      '保存できませんでした。2 MB未満の画像で再試行してください。',
    'wechat.unlink': 'WeChat連携を解除',
    'wechat.unlinkHint':
      'パスワードと有効な認証器、または確認済み電話番号で確認します。全端末からログアウトします。',
    'wechat.verifyUnlink': '確認して解除',
    'wechat.verifyPhone': '確認済み電話番号に送信',
    'wechat.unlinked': 'WeChat連携を解除しました。再ログインしてください。',
    'wechat.lastCredential':
      '別のログイン方法を追加してからWeChat連携を解除してください。',
    'wechat.unlinkError':
      'WeChat連携を解除できませんでした。アカウントを確認して再試行してください。',
    'wechat.confirm': 'WeChat連携を確認',
    'wechat.confirmHint':
      '確認済み電話番号はアカウント{{account}}と一致します。このWeChatを連携します。',
  },
  ru: {
    'wechat.editProfile': 'Изменить профиль',
    'wechat.cancelEdit': 'Отменить изменения',
    'wechat.displayProfile': 'Аватар и имя (необязательно)',
    'wechat.displayProfileHint':
      'Только для отображения. Можно пропустить и изменить позже в профиле.',
    'wechat.changeAvatar': 'Изменить аватар',
    'wechat.nicknameLabel': 'Имя WeChat (необязательно)',
    'wechat.nicknamePlaceholder': 'Выберите имя WeChat',
    'wechat.useWeChatNickname': 'Использовать имя WeChat',
    'wechat.nicknameAuthorizationHint':
      'Нажмите кнопку и выберите «Использовать имя WeChat» над клавиатурой WeChat. Этот шаг можно пропустить.',
    'wechat.platformRequiredTitle': 'Сначала привяжите номер на платформе',
    'wechat.platformRequiredHint':
      'На платформе нет аккаунта с этим подтверждённым номером. Подтвердите и привяжите его на платформе, затем вернитесь для входа.',
    'wechat.platformRequiredSteps':
      'Если аккаунта на платформе нет, сначала создайте его и привяжите номер.',
    'wechat.connectionHint':
      'Номер соответствует существующему аккаунту. Подтвердите привязку ниже.',
    'wechat.currentIdentity': 'Аккаунт WeChat',
    'wechat.currentWeChat': 'Текущий WeChat',
    'wechat.avatarOptional': 'Выбрать аватар (необязательно)',
    'wechat.platformAccount': 'Аккаунт платформы',
    'wechat.pendingConnection': 'Ожидает подтверждения',
    'wechat.phoneMatched': 'Подтверждённый номер совпадает',
    'wechat.confirmEffect':
      'После подтверждения этот WeChat сможет входить в указанный выше аккаунт платформы.',
    'wechat.additionalVerification':
      'Нужна дополнительная проверка. Введите пароль этого аккаунта и код аутентификатора, если он включён.',
    'wechat.confirmAndSignIn': 'Подтвердить и войти',
    'wechat.cancelConnection': 'Отменить привязку',
    'auth.error.appIdMismatch':
      'AppID мини-программы и сервера не совпадают. Попросите администратора проверить настройки WeChat.',
    'phone.login': 'Вход по SMS',
    'phone.description': 'Войдите с подтверждённым номером аккаунта.',
    'phone.number': 'Номер телефона (+86)',
    'phone.code': 'Код подтверждения SMS',
    'phone.send': 'Отправить SMS-код',
    'phone.resend': 'Повтор через {{seconds}} с',
    'phone.rateLimited': 'Слишком много запросов SMS. Повторите позже.',
    'phone.unavailable': 'SMS недоступны. Используйте другой способ входа.',
    'phone.invalidNumber': 'Введите действительный номер материкового Китая.',
    'phone.invalidCode':
      'Проверка не пройдена. Проверьте код или запросите новый.',
    'wechat.title': 'Привязка аккаунта WeChat',
    'wechat.bound': 'WeChat привязан',
    'wechat.notBound':
      'WeChat не привязан. Используйте вход по номеру WeChat для привязки.',
    'wechat.noProfile': 'Имя и аватар не указаны',
    'wechat.boundAt': 'Привязан: {{time}}',
    'wechat.profileHint':
      'Имя и аватар выбираются добровольно и не меняют аккаунт.',
    'wechat.chooseAvatar': 'Выбрать аватар',
    'wechat.nickname': 'Имя WeChat (необязательно)',
    'wechat.save': 'Сохранить профиль',
    'wechat.profileError':
      'Не удалось сохранить. Используйте изображение меньше 2 МБ.',
    'wechat.unlink': 'Отвязать WeChat',
    'wechat.unlinkHint':
      'Подтвердите пароль с аутентификатором или номер телефона. Все устройства выйдут из аккаунта.',
    'wechat.verifyUnlink': 'Подтвердить и отвязать',
    'wechat.verifyPhone': 'Отправить на подтверждённый номер',
    'wechat.unlinked': 'WeChat отвязан. Войдите снова.',
    'wechat.lastCredential':
      'Добавьте другой способ входа перед отвязкой WeChat.',
    'wechat.unlinkError':
      'Не удалось отвязать WeChat. Подтвердите аккаунт и повторите.',
    'wechat.confirm': 'Подтвердить привязку WeChat',
    'wechat.confirmHint':
      'Подтверждённый номер соответствует аккаунту {{account}}. Привяжите к нему WeChat.',
  },
  vi: {
    'wechat.editProfile': 'Sửa hồ sơ',
    'wechat.cancelEdit': 'Hủy chỉnh sửa',
    'wechat.displayProfile': 'Ảnh và biệt danh (tùy chọn)',
    'wechat.displayProfileHint':
      'Chỉ dùng để hiển thị. Bạn có thể bỏ qua và sửa sau trong hồ sơ.',
    'wechat.changeAvatar': 'Đổi ảnh đại diện',
    'wechat.nicknameLabel': 'Biệt danh WeChat (tùy chọn)',
    'wechat.nicknamePlaceholder': 'Chọn biệt danh WeChat',
    'wechat.useWeChatNickname': 'Sử dụng biệt danh WeChat',
    'wechat.nicknameAuthorizationHint':
      'Nhấn nút, sau đó chọn “Sử dụng biệt danh WeChat” phía trên bàn phím WeChat. Bạn có thể bỏ qua bước này.',
    'wechat.platformRequiredTitle':
      'Liên kết số điện thoại trên nền tảng trước',
    'wechat.platformRequiredHint':
      'Không có tài khoản nền tảng đã xác minh số này. Xác minh và liên kết số trên nền tảng, rồi quay lại đăng nhập.',
    'wechat.platformRequiredSteps':
      'Nếu chưa có tài khoản nền tảng, hãy tạo tài khoản và liên kết số điện thoại trước.',
    'wechat.connectionHint':
      'Số điện thoại khớp một tài khoản hiện có. Xác nhận liên kết bên dưới.',
    'wechat.currentIdentity': 'Tài khoản WeChat',
    'wechat.currentWeChat': 'WeChat hiện tại',
    'wechat.avatarOptional': 'Chạm để chọn ảnh (tùy chọn)',
    'wechat.platformAccount': 'Tài khoản nền tảng',
    'wechat.pendingConnection': 'Chờ xác nhận liên kết',
    'wechat.phoneMatched': 'Số đã xác minh khớp',
    'wechat.confirmEffect':
      'Sau khi xác nhận, WeChat này có thể đăng nhập tài khoản nền tảng ở trên.',
    'wechat.additionalVerification':
      'Tài khoản này cần xác minh thêm. Nhập mật khẩu và mã trình xác thực nếu đã bật.',
    'wechat.confirmAndSignIn': 'Xác nhận và đăng nhập',
    'wechat.cancelConnection': 'Hủy liên kết',
    'auth.error.appIdMismatch':
      'AppID của ứng dụng mini và máy chủ không khớp. Nhờ quản trị viên kiểm tra cấu hình WeChat.',
    'phone.login': 'Đăng nhập bằng SMS',
    'phone.description':
      'Đăng nhập bằng số điện thoại đã xác minh trên tài khoản.',
    'phone.number': 'Số điện thoại (+86)',
    'phone.code': 'Mã xác minh SMS',
    'phone.send': 'Gửi mã SMS',
    'phone.resend': 'Gửi lại sau {{seconds}} giây',
    'phone.rateLimited': 'Quá nhiều yêu cầu SMS. Vui lòng thử lại sau.',
    'phone.unavailable': 'SMS không khả dụng. Hãy dùng cách đăng nhập khác.',
    'phone.invalidNumber': 'Nhập số điện thoại hợp lệ tại Trung Quốc đại lục.',
    'phone.invalidCode': 'Xác minh thất bại. Kiểm tra mã hoặc yêu cầu mã mới.',
    'wechat.title': 'Liên kết tài khoản WeChat',
    'wechat.bound': 'Đã liên kết WeChat',
    'wechat.notBound':
      'Chưa liên kết WeChat. Đăng nhập bằng số WeChat để liên kết.',
    'wechat.noProfile': 'Chưa cung cấp biệt danh và ảnh đại diện',
    'wechat.boundAt': 'Liên kết lúc {{time}}',
    'wechat.profileHint':
      'Biệt danh và ảnh đại diện là tùy chọn, không thay đổi danh tính tài khoản.',
    'wechat.chooseAvatar': 'Chọn ảnh đại diện',
    'wechat.nickname': 'Biệt danh WeChat (tùy chọn)',
    'wechat.save': 'Lưu hồ sơ',
    'wechat.profileError':
      'Không thể lưu. Hãy dùng ảnh nhỏ hơn 2 MB và thử lại.',
    'wechat.unlink': 'Hủy liên kết WeChat',
    'wechat.unlinkHint':
      'Xác minh mật khẩu và trình xác thực, hoặc số đã liên kết. Mọi thiết bị sẽ đăng xuất.',
    'wechat.verifyUnlink': 'Xác minh và hủy liên kết',
    'wechat.verifyPhone': 'Gửi mã tới số đã xác minh',
    'wechat.unlinked': 'Đã hủy liên kết WeChat. Hãy đăng nhập lại.',
    'wechat.lastCredential':
      'Thêm cách đăng nhập khác trước khi hủy liên kết WeChat.',
    'wechat.unlinkError':
      'Không thể hủy liên kết WeChat. Xác minh tài khoản và thử lại.',
    'wechat.confirm': 'Xác nhận liên kết WeChat',
    'wechat.confirmHint':
      'Số đã xác minh khớp tài khoản {{account}}. Liên kết WeChat hiện tại với tài khoản này.',
  },
}
