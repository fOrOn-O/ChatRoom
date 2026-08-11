<template>
  <div class="workspace" :data-theme="theme" @click="closeMenus">
    <aside class="app-rail" aria-label="主导航">
      <button class="rail-logo" aria-label="ChatRoom 首页"><span></span><span></span><span></span></button>
      <nav class="rail-nav">
        <el-tooltip content="会话" placement="right"><button aria-label="会话" :class="{ active: activeSection === 'chats' }" @click.stop="activeSection = 'chats'"><el-icon><ChatDotRound /></el-icon><b v-if="totalUnread">{{ totalUnread > 99 ? '99+' : totalUnread }}</b></button></el-tooltip>
        <el-tooltip content="联系人" placement="right"><button aria-label="联系人" :class="{ active: activeSection === 'contacts' }" @click.stop="activeSection = 'contacts'"><el-icon><User /></el-icon></button></el-tooltip>
        <el-tooltip content="群组" placement="right"><button aria-label="群组" :class="{ active: activeSection === 'groups' }" @click.stop="activeSection = 'groups'"><el-icon><UserFilled /></el-icon></button></el-tooltip>
      </nav>
      <div class="rail-bottom">
        <el-tooltip content="个人资料" placement="right"><button aria-label="个人资料" @click.stop="openProfile"><el-avatar :size="30" :src="assetUrl(userStore.userInfo?.avatar)">{{ initial(userStore.userInfo?.nickname) }}</el-avatar></button></el-tooltip>
      </div>
    </aside>

    <aside class="conversation-pane">
      <header class="pane-head">
        <div class="identity">
          <span class="connection-dot" :class="{ connected }"></span>
          <div><strong>{{ userStore.userInfo?.nickname || 'ChatRoom' }}</strong><small role="status" aria-live="polite">{{ connected ? '已连接' : '正在连接…' }}</small></div>
        </div>
        <div class="pane-actions">
          <div class="theme-picker" @click.stop>
            <button
              class="icon-button theme-trigger"
              :class="{ selected: themeMenuOpen }"
              :aria-expanded="themeMenuOpen"
              :aria-label="`界面主题：${currentTheme.label}`"
              @click="toggleThemeMenu"
            >
              <el-icon><Brush /></el-icon>
            </button>
            <transition name="theme-menu">
              <div v-if="themeMenuOpen" class="theme-menu" role="radiogroup" aria-label="选择界面主题">
                <div class="theme-menu-head"><strong>界面主题</strong><span>选择工作台质感</span></div>
                <button
                  v-for="option in CHAT_THEMES"
                  :key="option.id"
                  class="theme-option"
                  :class="[`theme-option--${option.id}`, { active: theme === option.id }]"
                  role="radio"
                  :aria-checked="theme === option.id"
                  @click="selectTheme(option.id)"
                >
                  <span class="theme-swatch" aria-hidden="true"><i></i><i></i><i></i></span>
                  <span><strong>{{ option.label }}</strong><small>{{ option.description }}</small></span>
                  <el-icon class="theme-check"><Check /></el-icon>
                </button>
              </div>
            </transition>
          </div>
          <el-dropdown trigger="click" @command="handleAccountCommand">
            <button class="icon-button" aria-label="账户操作" @click.stop="themeMenuOpen = false"><el-icon><MoreFilled /></el-icon></button>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">编辑个人资料</el-dropdown-item>
                <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <div class="section-tabs" aria-label="内容分类">
        <button :class="{ active: activeSection === 'chats' }" @click="activeSection = 'chats'">消息</button>
        <button :class="{ active: activeSection === 'contacts' }" @click="activeSection = 'contacts'">联系人</button>
        <button :class="{ active: activeSection === 'groups' }" @click="activeSection = 'groups'">群组</button>
      </div>

      <div class="search-field">
        <el-icon><Search /></el-icon>
        <input v-model.trim="searchText" name="session-search" autocomplete="off" :placeholder="sectionSearchPlaceholder" aria-label="搜索当前列表">
        <button v-if="searchText" aria-label="清除搜索" @click="searchText = ''"><el-icon><Close /></el-icon></button>
      </div>

      <div class="list-heading">
        <span>{{ sectionTitle }}</span>
        <button v-if="activeSection === 'contacts'" class="small-action" @click="openUserSearch"><el-icon><Plus /></el-icon>添加</button>
        <button v-else-if="activeSection === 'groups'" class="small-action" @click="openCreateGroup"><el-icon><Plus /></el-icon>新建</button>
        <button v-else class="small-action" @click="openUserSearch"><el-icon><Plus /></el-icon>新对话</button>
      </div>

      <section class="session-list" aria-live="polite">
        <template v-if="filteredItems.length">
          <button
            v-for="item in filteredItems"
            :key="item.type + '_' + item.id"
            class="session-row"
            :class="{ active: isCurrent(item) }"
            @click="selectItem(item)"
            @contextmenu.prevent.stop="openContextMenu(item, $event)"
            @keydown="handleSessionKeydown(item, $event)"
          >
            <span class="avatar-wrap">
              <el-avatar :size="42" :src="assetUrl(item.avatar)">{{ initial(itemName(item)) }}</el-avatar>
              <i v-if="item.type === 'user' && item.online"></i>
              <span v-else-if="item.type === 'group'" class="group-avatar-mark"><el-icon><UserFilled /></el-icon></span>
            </span>
            <span class="row-copy">
              <span class="row-top"><strong>{{ itemName(item) }}</strong><time v-if="lastMessage(item)">{{ shortTime(lastMessage(item).timestamp) }}</time></span>
              <span class="row-bottom"><span>{{ activeSection === 'contacts' ? (item.online ? '在线' : '离线') : messagePreview(item) }}</span><em v-if="unread(item)">{{ unread(item) > 99 ? '99+' : unread(item) }}</em></span>
            </span>
          </button>
        </template>
        <div v-else class="list-empty">
          <el-icon><FolderOpened /></el-icon>
          <p>{{ searchText ? '没有匹配结果' : emptyListCopy }}</p>
          <button v-if="!searchText && activeSection !== 'chats'" @click="activeSection === 'contacts' ? openUserSearch() : openCreateGroup()">{{ activeSection === 'contacts' ? '添加联系人' : '创建第一个群组' }}</button>
        </div>
      </section>

      <div v-if="contextMenu.visible" ref="contextMenuRef" class="context-menu" role="menu" aria-label="会话操作" :style="{ left: contextMenu.left + 'px', top: contextMenu.top + 'px' }" @click.stop @keydown.esc.stop="closeMenus">
        <button role="menuitem" @click="contextMenu.visible = false">关闭菜单</button>
        <button v-if="contextMenu.item?.type === 'user'" role="menuitem" class="danger" @click="removeFriend(contextMenu.item)">删除联系人</button>
      </div>
    </aside>

    <main class="chat-stage">
      <template v-if="chatStore.currentChat">
        <header class="chat-header">
          <button class="mobile-back" aria-label="返回会话列表" @click="chatStore.currentChat = null"><el-icon><ArrowLeft /></el-icon></button>
          <div class="chat-title">
            <span class="avatar-wrap"><el-avatar :size="39" :src="assetUrl(chatStore.currentChat.avatar)">{{ initial(chatStore.currentChat.name) }}</el-avatar><i v-if="chatStore.currentChat.type === 'user' && currentOnline"></i></span>
            <div><h1>{{ chatStore.currentChat.name }}</h1><p>{{ chatSubtitle }}</p></div>
          </div>
          <button class="icon-button detail-button" :class="{ selected: inspectorOpen }" aria-label="打开会话详情" @click="toggleInspector"><el-icon><InfoFilled /></el-icon></button>
        </header>

        <div v-if="connectionNotice" class="connection-banner" role="status" aria-live="polite">
          <span class="connection-pulse" aria-hidden="true"></span>
          <div><strong>{{ connectionNotice.title }}</strong><span>{{ connectionNotice.description }}</span></div>
        </div>

        <div class="message-area">
          <section ref="messageListRef" class="messages" aria-label="聊天消息" @scroll="handleMessageScroll">
          <div v-if="historyLoading" class="history-state"><el-icon class="is-loading"><Loading /></el-icon> 正在加载消息</div>
          <template v-else>
            <div v-if="!currentMessages.length" class="conversation-empty">
              <div class="empty-orb"><el-icon><ChatLineRound /></el-icon></div>
              <strong>从一句问候开始</strong>
              <p>新消息会实时出现在这里。</p>
            </div>
            <transition-group v-else name="message" tag="div" class="message-stack">
              <div v-for="(message, index) in currentMessages" :key="message.msg_id" class="message-entry">
                <div v-if="showDateDivider(index)" class="date-divider"><span>{{ dateDivider(message.timestamp) }}</span></div>
                <article class="message-row" :class="{ self: isSelf(message), 'group-start': isMessageGroupStart(index), 'group-end': isMessageGroupEnd(index), grouped: !isMessageGroupStart(index) }">
                  <el-avatar v-if="!isSelf(message) && isMessageGroupStart(index)" :size="32" :src="messageAvatar(message)">{{ initial(messageSender(message)) }}</el-avatar>
                  <span v-else-if="!isSelf(message)" class="avatar-spacer" aria-hidden="true"></span>
                  <div class="message-body">
                    <div v-if="chatStore.currentChat.type === 'group' && !isSelf(message) && isMessageGroupStart(index)" class="sender-name">{{ messageSender(message) }}</div>
                    <div class="bubble">
                      <template v-if="message.content_type === 'image'">
                        <button class="image-preview-button" aria-label="预览聊天图片" @click="previewImage(message.content)"><img :src="assetUrl(message.content)" alt="发送的图片" class="image-message" width="330" height="220" loading="lazy"></button>
                      </template>
                      <template v-else-if="message.content_type === 'file'"><a :href="assetUrl(message.content)" class="file-message" target="_blank" rel="noopener" :download="fileName(message)" @click.prevent="downloadAttachment(message)"><el-icon><Document /></el-icon><span><strong>{{ fileName(message) }}</strong><small>点击下载文件</small></span><el-icon><Download /></el-icon></a></template>
                      <template v-else>{{ message.content }}</template>
                    </div>
                    <div v-if="isMessageGroupEnd(index)" class="message-meta"><time :datetime="isoTime(message.timestamp)">{{ fullTime(message.timestamp) }}</time><span v-if="isSelf(message) && message.local_status === 'sending'">发送中</span><span v-else-if="isSelf(message) && message.local_status === 'failed'" class="failed">未发送</span><span v-else-if="isSelf(message)">已发送</span></div>
                  </div>
                  <el-avatar v-if="isSelf(message) && isMessageGroupStart(index)" :size="32" :src="assetUrl(userStore.userInfo?.avatar)">{{ initial(userStore.userInfo?.nickname) }}</el-avatar>
                  <span v-else-if="isSelf(message)" class="avatar-spacer" aria-hidden="true"></span>
                </article>
              </div>
            </transition-group>
          </template>
          </section>
          <button v-if="showJumpToLatest" class="jump-latest" @click="jumpToLatest"><el-icon><ArrowDown /></el-icon>{{ unseenCurrentMessages ? unseenCurrentMessages + ' 条新消息' : '回到最新消息' }}</button>
        </div>
        <footer class="composer">
          <div class="composer-tools">
            <input ref="fileInputRef" class="file-input" name="attachment" type="file" accept="image/*,.pdf,.doc,.docx,.txt,.zip,.rar,.mp4,.webm" @change="uploadAttachment">
            <button :disabled="uploading" aria-label="发送文件" title="发送文件" @click="fileInputRef?.click()"><el-icon><Paperclip /></el-icon></button>
            <span>{{ uploading ? '正在上传文件…' : '支持图片、文档与压缩包，最大 50 MB' }}</span>
          </div>
          <textarea ref="composerRef" v-model="draft" name="message" autocomplete="off" :disabled="uploading" placeholder="输入消息…" aria-label="输入消息" @input="resizeComposer" @keydown="handleComposerKeydown"></textarea>
          <div class="composer-footer"><span>按 <kbd>Ctrl</kbd> + <kbd>Enter</kbd> 发送</span><button class="send-button" :disabled="!draft.trim() || !connected || uploading" @click="sendText">发送 <el-icon><Promotion /></el-icon></button></div>
        </footer>
      </template>

      <section v-else class="welcome-stage">
        <div class="welcome-art"><span></span><span></span><span></span></div>
        <p class="eyebrow">CHATROOM</p>
        <h1>把注意力留给重要的人。</h1>
        <p>从左侧选择一个联系人或群组，开始一段实时对话。</p>
        <button @click="openUserSearch"><el-icon><Plus /></el-icon>发起新对话</button>
      </section>
    </main>

    <aside v-if="inspectorOpen && chatStore.currentChat" class="inspector">
      <header><span>会话详情</span><button class="icon-button" aria-label="关闭详情" @click="inspectorOpen = false"><el-icon><Close /></el-icon></button></header>
      <div class="inspector-profile">
        <el-avatar :size="72" :src="assetUrl(chatStore.currentChat.avatar)">{{ initial(chatStore.currentChat.name) }}</el-avatar>
        <h2>{{ chatStore.currentChat.name }}</h2>
        <p v-if="chatStore.currentChat.type === 'user'">{{ currentOnline ? '现在在线' : '当前离线' }}</p>
        <p v-else>{{ groupMembers.length || '—' }} 位成员</p>
      </div>
      <template v-if="chatStore.currentChat.type === 'group'">
        <div class="detail-section"><span class="detail-label">群组说明</span><p>{{ groupInfo?.description || '群主还没有留下群说明。' }}</p></div>
        <div class="detail-section"><div class="detail-title"><span>成员</span><button v-if="isGroupOwner" @click="showInviteDialog = true"><el-icon><Plus /></el-icon>邀请</button></div><div class="member-list"><div v-for="member in groupMembers" :key="member.id" class="member-row"><el-avatar :size="30" :src="assetUrl(member.avatar)">{{ initial(member.nickname) }}</el-avatar><span>{{ member.nickname || member.username }}</span><em v-if="member.role === 2">群主</em><em v-else-if="member.role === 1">管理员</em><button v-if="isGroupOwner && member.role !== 2" :aria-label="`移除 ${member.nickname || member.username}`" title="移除成员" @click="removeMemberFromGroup(member)"><el-icon><Close /></el-icon></button></div></div></div>
        <div class="danger-zone"><button v-if="!isGroupOwner" @click="leaveCurrentGroup">退出此群组</button></div>
      </template>
      <template v-else><div class="detail-section"><span class="detail-label">账号</span><p>@{{ chatStore.currentChat.username || chatStore.currentChat.id }}</p></div><div class="danger-zone"><button @click="removeFriend(chatStore.currentChat)">删除联系人</button></div></template>
    </aside>

    <el-dialog v-model="showUserSearch" title="添加联系人" width="min(440px, calc(100% - 32px))" class="app-dialog">
      <p class="dialog-note">按用户名或昵称搜索。后端会直接建立双向好友关系。</p>
      <div class="dialog-search"><el-input v-model.trim="userKeyword" placeholder="输入用户名或昵称" @keyup.enter="searchUsers"><template #prefix><el-icon><Search /></el-icon></template></el-input><el-button :loading="searchingUsers" type="primary" @click="searchUsers">搜索</el-button></div>
      <div v-if="userResults.length" class="user-results"><div v-for="user in userResults" :key="user.id" class="user-result"><el-avatar :size="38" :src="assetUrl(user.avatar)">{{ initial(user.nickname) }}</el-avatar><div><strong>{{ user.nickname || user.username }}</strong><small>@{{ user.username }}</small></div><el-button :disabled="user.id === userStore.userInfo?.id || isFriend(user.id)" @click="addFriend(user)">{{ isFriend(user.id) ? '已是联系人' : user.id === userStore.userInfo?.id ? '当前账号' : '添加' }}</el-button></div></div>
      <el-empty v-else-if="hasSearchedUsers" description="没有找到匹配的用户" :image-size="90" />
    </el-dialog>

    <el-dialog v-model="showCreateGroup" title="创建群组" width="min(440px, calc(100% - 32px))" class="app-dialog">
      <el-form label-position="top" :model="groupForm"><el-form-item label="群组名称" required><el-input v-model.trim="groupForm.name" maxlength="100" show-word-limit placeholder="例如：周末计划" /></el-form-item><el-form-item label="群组说明"><el-input v-model.trim="groupForm.description" type="textarea" :rows="3" maxlength="500" show-word-limit placeholder="可选，用一句话介绍这个群组" /></el-form-item><el-form-item label="邀请联系人"><el-select v-model="groupForm.member_ids" multiple filterable placeholder="选择要邀请的人" style="width: 100%"><el-option v-for="friend in chatStore.friends" :key="friend.id" :label="friend.nickname || friend.username" :value="friend.id" /></el-select></el-form-item></el-form>
      <template #footer><el-button @click="showCreateGroup = false">取消</el-button><el-button type="primary" :loading="creatingGroup" @click="createNewGroup">创建群组</el-button></template>
    </el-dialog>

    <el-dialog v-model="showProfile" title="个人资料" width="min(440px, calc(100% - 32px))" class="app-dialog">
      <el-form label-position="top" :model="profileForm"><el-form-item label="用户名"><el-input :model-value="profileForm.username" disabled /></el-form-item><el-form-item label="显示名称"><el-input v-model.trim="profileForm.nickname" maxlength="50" /></el-form-item><el-form-item label="头像 URL"><el-input v-model.trim="profileForm.avatar" placeholder="可选，粘贴图片地址" /></el-form-item><el-form-item label="个人签名"><el-input v-model.trim="profileForm.signature" type="textarea" :rows="2" maxlength="255" show-word-limit /></el-form-item><div class="two-fields"><el-form-item label="邮箱"><el-input v-model.trim="profileForm.email" /></el-form-item><el-form-item label="手机"><el-input v-model.trim="profileForm.phone" /></el-form-item></div></el-form>
      <template #footer><el-button @click="showProfile = false">取消</el-button><el-button type="primary" :loading="savingProfile" @click="saveProfile">保存更改</el-button></template>
    </el-dialog>

    <el-dialog v-model="showInviteDialog" title="邀请群成员" width="min(400px, calc(100% - 32px))" class="app-dialog">
      <p class="dialog-note">选择要加入「{{ chatStore.currentChat?.name }}」的联系人。</p><el-select v-model="inviteIds" multiple filterable placeholder="选择联系人" style="width: 100%"><el-option v-for="friend in invitableFriends" :key="friend.id" :label="friend.nickname || friend.username" :value="friend.id" /></el-select>
      <template #footer><el-button @click="showInviteDialog = false">取消</el-button><el-button type="primary" :disabled="!inviteIds.length" @click="inviteToGroup">发送邀请</el-button></template>
    </el-dialog>

    <el-image-viewer v-if="previewingImage" :url-list="[previewingImage]" @close="previewingImage = ''" />
  </div>
</template>

<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '../../stores/user'
import { useChatStore } from '../../stores/chat'
import { updateProfile, searchUsers as searchUsersApi } from '../../api/user'
import { deleteFriend, sendFriendRequest } from '../../api/friend'
import { createGroup, getGroup, getGroupMembers, inviteMembers, leaveGroup, removeMember } from '../../api/group'
import { downloadFile, resolveStoredFileUrl, uploadFile } from '../../api/message'
import { connected, connect, disconnect, sendChatMessage, subscribe } from '../../websocket'
import { CHAT_THEMES, readStoredChatTheme, writeStoredChatTheme } from '../../theme'
import {
  canGroupMessages,
  formatDateDivider,
  formatFullTime,
  formatShortTime,
  isoTimestamp,
  shouldShowDateDivider
} from '../../utils/chatPresentation'

const router = useRouter()
const userStore = useUserStore()
const chatStore = useChatStore()

const theme = ref(readStoredChatTheme())
const themeMenuOpen = ref(false)
const activeSection = ref('chats')
const searchText = ref('')
const draft = ref('')
const chatDrafts = reactive({})
const historyLoading = ref(false)
const messageListRef = ref()
const composerRef = ref()
const fileInputRef = ref()
const contextMenuRef = ref()
const uploading = ref(false)
const inspectorOpen = ref(false)
const groupInfo = ref(null)
const groupMembers = ref([])
const previewingImage = ref('')
const showJumpToLatest = ref(false)
const unseenCurrentMessages = ref(0)
const sessionWasReplaced = ref(false)
const contextMenu = reactive({ visible: false, left: 0, top: 0, item: null })

const showUserSearch = ref(false)
const userKeyword = ref('')
const userResults = ref([])
const searchingUsers = ref(false)
const hasSearchedUsers = ref(false)

const showCreateGroup = ref(false)
const creatingGroup = ref(false)
const groupForm = reactive({ name: '', description: '', member_ids: [] })
const showInviteDialog = ref(false)
const inviteIds = ref([])

const showProfile = ref(false)
const savingProfile = ref(false)
const profileForm = reactive({ username: '', nickname: '', avatar: '', signature: '', email: '', phone: '' })
let composerResizeFrame
let unsubscribeCallbacks = []

const sectionTitle = computed(() => ({ chats: '最近会话', contacts: '联系人', groups: '我的群组' })[activeSection.value])
const sectionSearchPlaceholder = computed(() => ({ chats: '搜索会话…', contacts: '搜索联系人…', groups: '搜索群组…' })[activeSection.value])
const emptyListCopy = computed(() => ({ chats: '还没有可显示的会话', contacts: '还没有联系人', groups: '还没有加入群组' })[activeSection.value])
const totalUnread = computed(() => Object.values(chatStore.unreadCounts).reduce((sum, count) => sum + (count || 0), 0))
const currentTheme = computed(() => CHAT_THEMES.find((option) => option.id === theme.value) || CHAT_THEMES[0])
const currentMessages = computed(() => {
  const current = chatStore.currentChat
  return current ? chatStore.chatMessages[chatStore.chatKey(current.type, current.id)] || [] : []
})
const activeDraftKey = computed(() => {
  const current = chatStore.currentChat
  return current ? chatStore.chatKey(current.type, current.id) : ''
})
const currentOnline = computed(() => Boolean(chatStore.currentChat && chatStore.onlineUsers[chatStore.currentChat.id]))
const chatSubtitle = computed(() => {
  const current = chatStore.currentChat
  if (!current) return ''
  if (current.type === 'user') return currentOnline.value ? '在线 · 实时连接中' : '离线'
  return (groupMembers.value.length || current.member_count || '—') + ' 位成员'
})
const connectionNotice = computed(() => {
  if (sessionWasReplaced.value) {
    return { title: '连接已在其他页面打开', description: '此页面已停止自动重连，可刷新后重新进入。' }
  }
  if (!connected.value) {
    return { title: '正在恢复实时连接…', description: '连接恢复前暂时无法发送新消息。' }
  }
  return null
})
const isGroupOwner = computed(() => groupInfo.value?.owner_id === userStore.userInfo?.id)
const invitableFriends = computed(() => chatStore.friends.filter((friend) => !groupMembers.value.some((member) => member.id === friend.id)))
const filteredItems = computed(() => {
  const source = activeSection.value === 'contacts' ? chatStore.friends : activeSection.value === 'groups' ? chatStore.groups : [...chatStore.friends, ...chatStore.groups]
  const query = searchText.value.toLowerCase()
  if (!query) return source
  return source.filter((item) => [item.name, item.nickname, item.username].filter(Boolean).some((value) => value.toLowerCase().includes(query)))
})

function initial(value) {
  return (value || '?').trim().charAt(0).toUpperCase()
}

function assetUrl(url) {
  return resolveStoredFileUrl(url)
}

function itemName(item) {
  return item.name || item.nickname || item.username || '未命名会话'
}

function isCurrent(item) {
  return chatStore.currentChat?.type === item.type && chatStore.currentChat?.id === item.id
}

function keyFor(item) {
  return chatStore.chatKey(item.type, item.id)
}

function unread(item) {
  return chatStore.unreadCounts[keyFor(item)] || 0
}

function lastMessage(item) {
  const list = chatStore.chatMessages[keyFor(item)] || []
  return list[list.length - 1]
}

function messagePreview(item) {
  const message = lastMessage(item)
  if (!message) return item.type === 'group' ? '群组对话' : '开始一段对话'
  if (message.content_type === 'image') return '[图片]'
  if (message.content_type === 'file') return '[文件] ' + fileName(message)
  return message.content || '新消息'
}

function messageSender(message) {
  if (isSelf(message)) return userStore.userInfo?.nickname || '我'
  return message.from_name || '成员 #' + (message.from_id || message.from_user_id)
}

function messageAvatar(message) {
  if (isSelf(message)) return assetUrl(userStore.userInfo?.avatar)
  const user = chatStore.friends.find((friend) => friend.id === (message.from_id || message.from_user_id))
  return assetUrl(user?.avatar)
}

function fileName(message) {
  const raw = message.content || ''
  try {
    const url = new URL(raw, window.location.origin)
    if (url.searchParams.get('filename')) return url.searchParams.get('filename')
  } catch {
    // 保留兼容旧消息链接的降级逻辑。
  }
  const source = decodeURIComponent(raw.split('?')[0])
  const file = source.split('/').pop()
  return file || '附件文件'
}

const shortTime = formatShortTime
const fullTime = formatFullTime
const dateDivider = formatDateDivider
const isoTime = isoTimestamp

function isMessageGroupStart(index) {
  return !canGroupMessages(currentMessages.value[index - 1], currentMessages.value[index], userStore.userInfo?.id)
}

function isMessageGroupEnd(index) {
  return !canGroupMessages(currentMessages.value[index], currentMessages.value[index + 1], userStore.userInfo?.id)
}

function showDateDivider(index) {
  return shouldShowDateDivider(currentMessages.value, index)
}

async function selectItem(item) {
  closeMenus()
  showJumpToLatest.value = false
  unseenCurrentMessages.value = 0
  const chat = { ...item, name: itemName(item) }
  chatStore.setCurrentChat(chat)
  activeSection.value = item.type === 'group' ? 'groups' : 'chats'
  inspectorOpen.value = false
  historyLoading.value = true
  try {
    await chatStore.fetchHistory(item.id, item.type)
    if (item.type === 'group') await loadGroupDetails(item.id)
    await nextTick()
    scrollToBottom()
  } finally {
    historyLoading.value = false
  }
}

async function loadGroupDetails(groupId) {
  const [groupResponse, membersResponse] = await Promise.all([getGroup(groupId), getGroupMembers(groupId)])
  groupInfo.value = groupResponse.data
  groupMembers.value = membersResponse.data || []
}

async function toggleInspector() {
  inspectorOpen.value = !inspectorOpen.value
  if (inspectorOpen.value && chatStore.currentChat?.type === 'group') await loadGroupDetails(chatStore.currentChat.id)
}

function isNearLatest() {
  const node = messageListRef.value
  return !node || node.scrollHeight - node.scrollTop - node.clientHeight <= 80
}

function scrollToBottom(behavior = 'auto') {
  nextTick(() => {
    const node = messageListRef.value
    if (!node) return
    node.scrollTo({ top: node.scrollHeight, behavior })
  })
}

function jumpToLatest() {
  showJumpToLatest.value = false
  unseenCurrentMessages.value = 0
  scrollToBottom('smooth')
}

function handleMessageScroll() {
  const node = messageListRef.value
  if (!node) return
  if (isNearLatest()) {
    showJumpToLatest.value = false
    unseenCurrentMessages.value = 0
  } else {
    showJumpToLatest.value = node.scrollHeight - node.scrollTop - node.clientHeight > 180
  }
}

function isSelf(message) {
  return message.from_id === userStore.userInfo?.id || message.from_user_id === userStore.userInfo?.id
}

function queueMessage(contentType, content) {
  const current = chatStore.currentChat
  if (!current) return false
  const msgId = sendChatMessage({ toId: current.id, toType: current.type, contentType, content })
  if (!msgId) {
    ElMessage.warning('实时连接尚未建立，请稍后重试')
    return false
  }
  chatStore.addMessage({
    msg_id: msgId,
    from_id: userStore.userInfo.id,
    from_name: userStore.userInfo.nickname,
    to_id: current.id,
    to_type: current.type,
    content_type: contentType,
    content,
    timestamp: Date.now(),
    local_status: 'sending'
  }, { incrementUnread: false })
  scrollToBottom()
  return true
}

function sendText() {
  const content = draft.value.trim()
  if (!content || uploading.value) return
  if (queueMessage('text', content)) draft.value = ''
  composerRef.value?.focus()
}

function handleComposerKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    sendText()
  }
}

function resizeComposer() {
  window.cancelAnimationFrame(composerResizeFrame)
  composerResizeFrame = window.requestAnimationFrame(() => {
    const node = composerRef.value
    if (!node) return
    node.style.height = 'auto'
    node.style.height = Math.min(Math.max(node.scrollHeight, 56), 160) + 'px'
  })
}

async function uploadAttachment(event) {
  const file = event.target.files?.[0]
  if (!file || !chatStore.currentChat) return
  if (file.size > 50 * 1024 * 1024) {
    ElMessage.error('文件大小不能超过 50 MB')
    event.target.value = ''
    return
  }
  uploading.value = true
  try {
    const response = await uploadFile(file)
    const data = response.data
    const isImage = data.mimetype?.startsWith('image/')
    const content = isImage
      ? assetUrl(data.url)
      : '/api/v1/files/' + data.id + '/download?filename=' + encodeURIComponent(data.filename || '附件文件')
    queueMessage(isImage ? 'image' : 'file', content)
  } finally {
    uploading.value = false
    event.target.value = ''
  }
}

async function downloadAttachment(message) {
  const matched = (message.content || '').match(/\/files\/(\d+)\/download/)
  if (!matched) {
    window.open(assetUrl(message.content), '_blank', 'noopener')
    return
  }
  const response = await downloadFile(matched[1])
  const objectUrl = URL.createObjectURL(response.data)
  const link = document.createElement('a')
  link.href = objectUrl
  link.download = fileName(message)
  document.body.appendChild(link)
  link.click()
  link.remove()
  URL.revokeObjectURL(objectUrl)
}

function previewImage(url) {
  previewingImage.value = assetUrl(url)
}

function openContextMenu(item, event) {
  contextMenu.item = item
  contextMenu.left = Math.min(event.clientX, window.innerWidth - 165)
  contextMenu.top = Math.min(event.clientY, window.innerHeight - 90)
  contextMenu.visible = true
  nextTick(() => contextMenuRef.value?.querySelector('button')?.focus())
}

function handleSessionKeydown(item, event) {
  if (event.key !== 'ContextMenu' && !(event.shiftKey && event.key === 'F10')) return
  event.preventDefault()
  event.stopPropagation()
  const rect = event.currentTarget.getBoundingClientRect()
  openContextMenu(item, { clientX: rect.left + 24, clientY: rect.top + 24 })
}

function closeMenus() {
  contextMenu.visible = false
  themeMenuOpen.value = false
}

function toggleThemeMenu() {
  contextMenu.visible = false
  themeMenuOpen.value = !themeMenuOpen.value
}

function selectTheme(nextTheme) {
  theme.value = writeStoredChatTheme(nextTheme)
  themeMenuOpen.value = false
}

function openUserSearch() {
  closeMenus()
  showUserSearch.value = true
  userKeyword.value = ''
  userResults.value = []
  hasSearchedUsers.value = false
  nextTick(() => document.querySelector('.dialog-search input')?.focus())
}

async function searchUsers() {
  if (!userKeyword.value) {
    ElMessage.warning('请输入用户名或昵称')
    return
  }
  searchingUsers.value = true
  try {
    const response = await searchUsersApi(userKeyword.value)
    userResults.value = response.data || []
    hasSearchedUsers.value = true
  } finally {
    searchingUsers.value = false
  }
}

function isFriend(userId) {
  return chatStore.friends.some((friend) => friend.id === userId)
}

async function addFriend(user) {
  await sendFriendRequest({ user_id: user.id })
  await chatStore.fetchFriends()
  ElMessage.success('已添加 ' + (user.nickname || user.username))
  showUserSearch.value = false
}

function openCreateGroup() {
  closeMenus()
  groupForm.name = ''
  groupForm.description = ''
  groupForm.member_ids = []
  showCreateGroup.value = true
}

async function createNewGroup() {
  if (!groupForm.name) {
    ElMessage.warning('请输入群组名称')
    return
  }
  creatingGroup.value = true
  try {
    const response = await createGroup({ ...groupForm })
    await chatStore.fetchGroups()
    showCreateGroup.value = false
    ElMessage.success('群组创建成功')
    const group = chatStore.groups.find((item) => item.id === response.data.id)
    if (group) selectItem(group)
  } finally {
    creatingGroup.value = false
  }
}

async function inviteToGroup() {
  const current = chatStore.currentChat
  if (!current || !inviteIds.value.length) return
  await inviteMembers(current.id, { user_ids: inviteIds.value })
  inviteIds.value = []
  showInviteDialog.value = false
  await loadGroupDetails(current.id)
  ElMessage.success('成员已添加')
}

async function removeMemberFromGroup(member) {
  const current = chatStore.currentChat
  if (!current) return
  try {
    await ElMessageBox.confirm('确定将 ' + (member.nickname || member.username) + ' 移出群组？', '移除成员', { type: 'warning' })
    await removeMember(current.id, member.id)
    await loadGroupDetails(current.id)
    ElMessage.success('成员已移除')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

async function leaveCurrentGroup() {
  const current = chatStore.currentChat
  if (!current) return
  try {
    await ElMessageBox.confirm('退出后将不再接收该群组的新消息。', '退出群组？', { type: 'warning' })
    await leaveGroup(current.id)
    chatStore.removeGroup(current.id)
    inspectorOpen.value = false
    ElMessage.success('已退出群组')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

async function removeFriend(friend) {
  if (!friend?.id) return
  closeMenus()
  try {
    await ElMessageBox.confirm('删除后将无法继续使用该联系人会话。', '删除联系人？', { type: 'warning' })
    await deleteFriend(friend.id)
    chatStore.removeFriend(friend.id)
    inspectorOpen.value = false
    ElMessage.success('联系人已删除')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

function openProfile() {
  Object.assign(profileForm, {
    username: userStore.userInfo?.username || '',
    nickname: userStore.userInfo?.nickname || '',
    avatar: userStore.userInfo?.avatar || '',
    signature: userStore.userInfo?.signature || '',
    email: userStore.userInfo?.email || '',
    phone: userStore.userInfo?.phone || ''
  })
  showProfile.value = true
}

async function saveProfile() {
  savingProfile.value = true
  try {
    await updateProfile({
      nickname: profileForm.nickname,
      avatar: profileForm.avatar,
      signature: profileForm.signature,
      email: profileForm.email,
      phone: profileForm.phone
    })
    await userStore.fetchProfile()
    showProfile.value = false
    ElMessage.success('个人资料已更新')
  } finally {
    savingProfile.value = false
  }
}

function handleAccountCommand(command) {
  if (command === 'profile') openProfile()
  if (command === 'logout') {
    disconnect()
    userStore.logout()
    router.replace('/login')
  }
}

watch(() => chatStore.currentChat?.id, () => {
  inviteIds.value = []
  showJumpToLatest.value = false
  unseenCurrentMessages.value = 0
})
watch(activeDraftKey, (nextKey, previousKey) => {
  if (previousKey) chatDrafts[previousKey] = draft.value
  draft.value = nextKey ? chatDrafts[nextKey] || '' : ''
  nextTick(resizeComposer)
})
watch(draft, (value) => {
  if (activeDraftKey.value) chatDrafts[activeDraftKey.value] = value
})
watch(connected, (isConnected) => {
  if (isConnected) sessionWasReplaced.value = false
})
watch(theme, (value) => {
  document.documentElement.dataset.chatTheme = value
}, { immediate: true })

onMounted(async () => {
  try {
    await userStore.fetchProfile()
    unsubscribeCallbacks = [
      subscribe('message', (message) => {
        const shouldFollowLatest = isNearLatest()
        const key = chatStore.addMessage(message)
        if (key === (chatStore.currentChat && chatStore.chatKey(chatStore.currentChat.type, chatStore.currentChat.id))) {
          if (shouldFollowLatest) {
            scrollToBottom()
          } else {
            unseenCurrentMessages.value += 1
            showJumpToLatest.value = true
          }
        }
      }),
      subscribe('ack', (ack) => {
        Object.values(chatStore.chatMessages).forEach((list) => {
          const message = list.find((item) => item.msg_id === ack.msg_id)
          if (message) message.local_status = ack.status === 'sent' ? 'sent' : message.local_status
        })
      }),
      subscribe('onlineStatus', (status) => chatStore.setOnlineStatus(status.user_id, status.online)),
      subscribe('sessionReplaced', () => {
        sessionWasReplaced.value = true
        ElMessage.warning('当前连接已被新会话替换，已停止自动重连')
      })
    ]
    connect(userStore.token)
    await Promise.all([chatStore.fetchFriends(), chatStore.fetchGroups()])
  } catch {
    // API 拦截器已经向用户展示了可读的错误信息。
  }
})

onBeforeUnmount(() => {
  unsubscribeCallbacks.forEach((unsubscribe) => unsubscribe())
  window.cancelAnimationFrame(composerResizeFrame)
  disconnect()
  delete document.documentElement.dataset.chatTheme
})
</script>

<style scoped>
.workspace { --forest:#19362c; --forest-soft:#28543d; --mint:#dcefd0; --lime:#bfe879; --ink:#1d2821; --muted:#7a887e; --line:#e6ebe4; display:grid; grid-template-columns:64px 306px minmax(0,1fr); min-height:100svh; overflow:hidden; color:var(--ink); background:#f7f8f5; }
.app-rail { display:flex; flex-direction:column; align-items:center; padding:16px 0; color:#b6c8b9; background:var(--forest); }.rail-logo { display:flex; align-items:flex-end; justify-content:center; gap:3px; width:34px; height:34px; padding:0 7px 7px; border:0; border-radius:10px; cursor:pointer; background:var(--mint); }.rail-logo span { width:4px; border-radius:10px; background:var(--forest); }.rail-logo span:nth-child(1){height:9px}.rail-logo span:nth-child(2){height:16px}.rail-logo span:nth-child(3){height:22px}.rail-nav { display:grid; gap:10px; margin-top:43px; }.rail-nav button,.rail-bottom button { position:relative; display:grid; place-items:center; width:42px; height:42px; border:0; border-radius:11px; color:#b6c8b9; cursor:pointer; background:transparent; transition:color .18s,background .18s,transform .18s; }.rail-nav button:hover,.rail-bottom button:hover { color:#fff; background:rgba(255,255,255,.09); }.rail-nav button.active { color:var(--forest); background:var(--mint); }.rail-nav .el-icon { font-size:20px; }.rail-nav b { position:absolute; top:-4px; right:-7px; min-width:17px; padding:2px 4px; border:2px solid var(--forest); border-radius:10px; color:#fff; background:#e96061; font-size:9px; line-height:1; }.rail-bottom { margin-top:auto; }.rail-bottom button { overflow:hidden; }
.conversation-pane { position:relative; display:flex; flex-direction:column; min-width:0; border-right:1px solid var(--line); background:#fff; }.pane-head { display:flex; align-items:center; justify-content:space-between; min-height:76px; padding:0 18px 0 20px; }.identity { display:flex; align-items:center; gap:10px; }.identity strong { display:block; font-size:13px; line-height:1.3; }.identity small { display:block; margin-top:2px; color:var(--muted); font-size:11px; }.connection-dot { width:8px; height:8px; border-radius:50%; background:#b9c2ba; }.connection-dot.connected { background:#65af71; box-shadow:0 0 0 4px rgba(101,175,113,.11); }.icon-button { display:grid; place-items:center; width:32px; height:32px; border:0; border-radius:8px; color:#7c897f; cursor:pointer; background:transparent; transition:background .18s,color .18s; }.icon-button:hover,.icon-button.selected { color:#244630; background:#edf3e9; }.icon-button .el-icon { font-size:18px; }.section-tabs { display:flex; gap:4px; padding:0 15px 14px; }.section-tabs button { flex:1; height:32px; border:0; border-radius:7px; color:#87938a; cursor:pointer; background:transparent; font-size:12px; font-weight:700; }.section-tabs button:hover { color:#355b40; background:#f4f7f2; }.section-tabs button.active { color:#28583c; background:#eaf4e5; }.search-field { display:flex; align-items:center; gap:8px; height:36px; margin:0 17px; padding:0 10px; border:1px solid #e6ece3; border-radius:8px; color:#91a096; background:#f8faf7; transition:border .18s,background .18s; }.search-field:focus-within { border-color:#8ab38b; background:#fff; }.search-field .el-icon { font-size:15px; }.search-field input { width:100%; min-width:0; border:0; outline:0; color:#26332b; background:transparent; font-size:12px; }.search-field input::placeholder { color:#9aa59d; }.search-field button { display:grid; place-items:center; padding:0; border:0; color:#98a39b; cursor:pointer; background:transparent; }.list-heading { display:flex; align-items:center; justify-content:space-between; padding:24px 20px 10px; color:#69776e; font-size:11px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }.small-action { display:inline-flex; align-items:center; gap:3px; padding:3px 0; border:0; color:#397044; cursor:pointer; background:none; font-size:11px; font-weight:800; }.small-action:hover { color:#173f25; }.small-action .el-icon { font-size:13px; }.session-list { flex:1; overflow-y:auto; padding:0 9px 15px; }.session-row { display:flex; align-items:center; gap:10px; width:100%; padding:10px 10px; border:0; border-radius:10px; color:inherit; cursor:pointer; text-align:left; background:transparent; transition:background .16s,transform .16s; }.session-row:hover { background:#f5f8f3; }.session-row.active { background:#eaf4e5; }.avatar-wrap { position:relative; display:inline-grid; flex:0 0 auto; place-items:center; }.avatar-wrap > i { position:absolute; right:-1px; bottom:0; width:10px; height:10px; border:2px solid #fff; border-radius:50%; background:#65af71; }.group-avatar-mark { position:absolute; right:-4px; bottom:-3px; display:grid; place-items:center; width:14px; height:14px; border:1px solid #fff; border-radius:50%; color:#477250; background:#d9ebd4; }.group-avatar-mark .el-icon { font-size:8px; }.row-copy { min-width:0; flex:1; }.row-top,.row-bottom { display:flex; align-items:center; justify-content:space-between; gap:6px; }.row-top strong { overflow:hidden; color:#2a382f; font-size:13px; font-weight:700; text-overflow:ellipsis; white-space:nowrap; }.row-top time { flex:0 0 auto; color:#a0aca2; font-size:10px; }.row-bottom { margin-top:4px; }.row-bottom > span { overflow:hidden; color:#929d94; font-size:11px; text-overflow:ellipsis; white-space:nowrap; }.row-bottom em { display:grid; flex:0 0 auto; place-items:center; min-width:16px; height:16px; padding:0 4px; border-radius:8px; color:#fff; background:#4d9457; font-size:9px; font-style:normal; font-weight:800; }.list-empty { display:grid; place-items:center; align-content:center; min-height:210px; padding:20px; color:#9aa59d; text-align:center; }.list-empty .el-icon { font-size:30px; color:#b7c5b8; }.list-empty p { margin:10px 0; font-size:12px; }.list-empty button { border:0; color:#397044; cursor:pointer; background:none; font-size:12px; font-weight:800; }.context-menu { position:fixed; z-index:50; min-width:145px; padding:5px; border:1px solid #e6ebe4; border-radius:9px; box-shadow:0 14px 30px rgba(23,45,30,.13); background:#fff; }.context-menu button { display:block; width:100%; padding:8px 10px; border:0; border-radius:6px; color:#59675d; cursor:pointer; text-align:left; background:transparent; font-size:12px; }.context-menu button:hover { background:#f4f7f2; }.context-menu .danger { color:#bd4e52; }
.chat-stage { display:flex; flex-direction:column; min-width:0; min-height:0; background:#f8faf7; }.chat-header { display:flex; align-items:center; justify-content:space-between; min-height:76px; padding:0 28px; border-bottom:1px solid var(--line); background:rgba(255,255,255,.74); }.mobile-back { display:none; }.chat-title { display:flex; align-items:center; gap:11px; min-width:0; }.chat-title h1 { overflow:hidden; margin:0; color:#26342b; font-size:15px; letter-spacing:-.02em; text-overflow:ellipsis; white-space:nowrap; }.chat-title p { margin:3px 0 0; color:#8b988f; font-size:11px; }.detail-button { width:35px; height:35px; }.messages { flex:1; min-height:0; overflow-y:auto; padding:28px clamp(22px,5vw,78px); scroll-behavior:smooth; }.message-stack { display:flex; flex-direction:column; gap:22px; max-width:880px; margin:auto; }.message-row { display:flex; align-items:flex-start; gap:9px; }.message-row.self { flex-direction:row-reverse; }.message-body { max-width:min(74%,570px); }.sender-name { margin:0 0 4px 3px; color:#809086; font-size:11px; }.bubble { min-width:48px; padding:10px 13px; border:1px solid #e5ebe3; border-radius:4px 15px 15px 15px; color:#27352c; background:#fff; box-shadow:0 2px 4px rgba(34,55,39,.025); font-size:13px; line-height:1.65; word-break:break-word; }.self .bubble { border-color:#d6e7c9; border-radius:15px 4px 15px 15px; color:#24402b; background:#e0f1d4; }.message-meta { display:flex; align-items:center; gap:6px; margin:4px 3px 0; color:#a1aaa3; font-size:10px; }.self .message-meta { justify-content:flex-end; }.message-meta .failed { color:#c65c5e; }.image-message { display:block; max-width:min(330px,60vw); max-height:300px; border-radius:9px; cursor:zoom-in; object-fit:contain; }.file-message { display:flex; align-items:center; gap:10px; min-width:190px; color:#315d3a; text-decoration:none; }.file-message > .el-icon:first-child { display:grid; flex:0 0 auto; place-items:center; width:32px; height:32px; border-radius:8px; color:#477d50; background:#d5ebcc; font-size:17px; }.file-message span { min-width:0; flex:1; }.file-message strong,.file-message small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.file-message strong { font-size:12px; }.file-message small { margin-top:1px; color:#7f9581; font-size:10px; }.file-message > .el-icon:last-child { font-size:15px; }.history-state { display:flex; justify-content:center; gap:7px; padding:25px; color:#89968c; font-size:12px; }.conversation-empty { display:grid; place-items:center; align-content:center; min-height:100%; color:#87958b; text-align:center; }.empty-orb { display:grid; place-items:center; width:52px; height:52px; margin-bottom:13px; border-radius:20px; color:#477854; background:#e5f0df; }.empty-orb .el-icon { font-size:24px; }.conversation-empty strong { color:#4c5e51; font-size:14px; }.conversation-empty p { margin:6px 0; font-size:12px; }.composer { padding:13px clamp(22px,5vw,78px) 16px; border-top:1px solid var(--line); background:#fff; }.composer-tools { display:flex; align-items:center; gap:8px; min-height:27px; color:#94a098; font-size:10px; }.composer-tools button { display:grid; place-items:center; width:27px; height:25px; padding:0; border:0; border-radius:6px; color:#617568; cursor:pointer; background:transparent; }.composer-tools button:hover { color:#28583c; background:#eef5ea; }.composer-tools button:disabled { opacity:.5; cursor:not-allowed; }.composer-tools .el-icon { font-size:17px; }.file-input { display:none; }.composer textarea { display:block; width:100%; min-height:56px; max-height:160px; margin:3px 0; padding:5px 0; border:0; outline:0; resize:vertical; color:#27352b; background:transparent; font-size:13px; line-height:1.6; }.composer textarea::placeholder { color:#a7b0a9; }.composer-footer { display:flex; align-items:center; justify-content:space-between; color:#9ba69e; font-size:10px; }.composer-footer kbd { padding:1px 4px; border:1px solid #dbe2da; border-radius:3px; background:#f6f8f5; font-family:inherit; font-size:9px; }.send-button { display:inline-flex; align-items:center; gap:6px; height:30px; padding:0 13px; border:0; border-radius:7px; color:#f6faf4; cursor:pointer; background:#315f3d; font-size:11px; font-weight:800; transition:background .16s,transform .16s; }.send-button:hover:not(:disabled) { background:#1e4a2c; transform:translateY(-1px); }.send-button:disabled { color:#a7b2a8; cursor:not-allowed; background:#e7ebe6; }
.welcome-stage { display:grid; flex:1; place-items:center; align-content:center; padding:35px; color:#728076; text-align:center; }.welcome-art { position:relative; width:104px; height:88px; margin-bottom:20px; }.welcome-art span { position:absolute; display:block; border:2px solid #5d9367; border-radius:16px; background:#e7f3df; }.welcome-art span:nth-child(1) { top:3px; left:7px; width:69px; height:51px; }.welcome-art span:nth-child(2) { right:2px; bottom:1px; width:62px; height:47px; border-color:#8db172; background:#f4faef; }.welcome-art span:nth-child(3) { bottom:11px; left:0; width:18px; height:18px; border:0; border-radius:50%; background:#bcdf91; }.welcome-stage .eyebrow { color:#76a679; }.welcome-stage h1 { max-width:500px; margin:12px 0 8px; color:#33453a; font-size:clamp(24px,3vw,34px); letter-spacing:-.055em; }.welcome-stage > p:last-of-type { max-width:350px; margin:0; font-size:13px; line-height:1.7; }.welcome-stage button { display:inline-flex; align-items:center; gap:5px; margin-top:23px; padding:9px 13px; border:1px solid #cce0c8; border-radius:8px; color:#315f3d; cursor:pointer; background:#fff; font-size:12px; font-weight:800; }.welcome-stage button:hover { background:#eef6ea; }
.inspector { display:flex; flex-direction:column; width:286px; min-height:0; border-left:1px solid var(--line); background:#fff; animation:inspector-in .2s ease-out; }.inspector > header { display:flex; align-items:center; justify-content:space-between; min-height:76px; padding:0 18px; border-bottom:1px solid var(--line); color:#56655a; font-size:12px; font-weight:800; }.inspector-profile { padding:25px 18px 22px; border-bottom:1px solid var(--line); text-align:center; }.inspector-profile h2 { margin:10px 0 3px; color:#314038; font-size:16px; letter-spacing:-.03em; }.inspector-profile p { margin:0; color:#8a978d; font-size:11px; }.detail-section { padding:19px 18px; border-bottom:1px solid #eef1ed; }.detail-label { color:#87958a; font-size:10px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }.detail-section > p { margin:8px 0 0; color:#506055; font-size:12px; line-height:1.7; }.detail-title { display:flex; align-items:center; justify-content:space-between; color:#65736a; font-size:11px; font-weight:800; }.detail-title button { display:inline-flex; align-items:center; gap:2px; padding:0; border:0; color:#3d7548; cursor:pointer; background:none; font-size:11px; font-weight:800; }.member-list { max-height:220px; margin-top:11px; overflow-y:auto; }.member-row { display:flex; align-items:center; gap:8px; min-height:39px; }.member-row > span { overflow:hidden; flex:1; color:#4d5d52; font-size:12px; text-overflow:ellipsis; white-space:nowrap; }.member-row em { color:#7c9b70; font-size:10px; font-style:normal; }.member-row button { display:grid; place-items:center; padding:3px; border:0; color:#a5aea6; cursor:pointer; background:transparent; }.member-row button:hover { color:#be595c; }.danger-zone { margin-top:auto; padding:16px 18px; }.danger-zone button { width:100%; height:34px; border:1px solid #f0d4d5; border-radius:7px; color:#b95659; cursor:pointer; background:#fffafa; font-size:11px; font-weight:800; }.danger-zone button:hover { background:#fff1f1; }@keyframes inspector-in{from{opacity:0;transform:translateX(12px)}to{opacity:1;transform:translateX(0)}}.message-enter-active{transition:opacity .22s ease,transform .22s ease}.message-enter-from{opacity:0;transform:translateY(8px)}
.dialog-note { margin:0 0 16px; color:#758278; font-size:12px; line-height:1.6; }.dialog-search { display:flex; gap:9px; }.user-results { margin-top:14px; }.user-result { display:flex; align-items:center; gap:10px; padding:10px 0; border-bottom:1px solid #edf1ec; }.user-result > div { min-width:0; flex:1; }.user-result strong,.user-result small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.user-result strong { color:#35443a; font-size:13px; }.user-result small { margin-top:2px; color:#91a097; font-size:11px; }.user-result :deep(.el-button) { border-radius:7px; font-size:12px; }.two-fields { display:grid; grid-template-columns:1fr 1fr; gap:12px; }.app-dialog :deep(.el-dialog) { border-radius:13px; }.app-dialog :deep(.el-dialog__header) { margin-right:0; padding:20px 22px 11px; }.app-dialog :deep(.el-dialog__title) { color:#344238; font-size:16px; font-weight:800; }.app-dialog :deep(.el-dialog__body) { padding:13px 22px 17px; }.app-dialog :deep(.el-dialog__footer) { padding:10px 22px 19px; }.app-dialog :deep(.el-form-item) { margin-bottom:14px; }.app-dialog :deep(.el-form-item__label) { padding-bottom:5px; color:#66736a; font-size:12px; font-weight:700; }.app-dialog :deep(.el-input__wrapper),.app-dialog :deep(.el-textarea__inner),.app-dialog :deep(.el-select__wrapper) { border-radius:8px; box-shadow:0 0 0 1px #e0e7df inset; }.app-dialog :deep(.el-button--primary) { border-color:#315f3d; background:#315f3d; }

/* Theme controls */
.pane-actions { display:flex; align-items:center; gap:2px; }
.theme-picker { position:relative; }
.theme-trigger { position:relative; }
.theme-trigger::after { position:absolute; right:4px; bottom:4px; width:5px; height:5px; border:1px solid #fff; border-radius:50%; background:#72a96f; content:''; }
.theme-menu { position:absolute; z-index:70; top:41px; right:0; width:252px; padding:10px; border:1px solid #e2e8e0; border-radius:14px; background:rgba(255,255,255,.98); box-shadow:0 18px 45px rgba(25,54,44,.16); transform-origin:top right; }
.theme-menu-head { display:flex; align-items:baseline; justify-content:space-between; padding:3px 5px 9px; }
.theme-menu-head strong { color:#2d3d33; font-size:12px; }
.theme-menu-head span { color:#98a39b; font-size:10px; }
.theme-option { display:grid; grid-template-columns:46px minmax(0,1fr) 16px; align-items:center; gap:9px; width:100%; min-height:52px; padding:7px; border:0; border-radius:10px; color:#35443a; cursor:pointer; text-align:left; background:transparent; transition:background .18s,transform .18s; }
.theme-option:hover { background:#f3f6f1; transform:translateX(2px); }
.theme-option.active { background:#eaf3e6; }
.theme-option > span:nth-child(2) { min-width:0; }
.theme-option strong,.theme-option small { display:block; }
.theme-option strong { font-size:12px; }
.theme-option small { overflow:hidden; margin-top:2px; color:#89968d; font-size:9px; text-overflow:ellipsis; white-space:nowrap; }
.theme-swatch { position:relative; display:flex; align-items:flex-end; justify-content:center; gap:3px; width:44px; height:34px; padding:7px; overflow:hidden; border-radius:8px; }
.theme-swatch i { display:block; width:5px; border-radius:5px; background:#356343; }
.theme-swatch i:nth-child(1) { height:8px; }.theme-swatch i:nth-child(2) { height:14px; }.theme-swatch i:nth-child(3) { height:19px; }
.theme-option--minimal .theme-swatch { border:1px solid #e0e7df; background:#fff; }
.theme-option--glass .theme-swatch { border:1px solid rgba(255,255,255,.72); background:linear-gradient(135deg,rgba(219,250,236,.85),rgba(209,221,252,.68) 48%,rgba(249,221,232,.72)); box-shadow:inset 0 0 12px rgba(255,255,255,.7); }
.theme-option--neumorphic .theme-swatch { background:#e7ece7; box-shadow:3px 3px 6px #c7cec8,-3px -3px 6px #fff; }
.theme-check { color:#3e7849; opacity:0; font-size:14px; }
.theme-option.active .theme-check { opacity:1; }
.theme-menu-enter-active,.theme-menu-leave-active { transition:opacity .16s ease,transform .16s ease; }
.theme-menu-enter-from,.theme-menu-leave-to { opacity:0; transform:translateY(-5px) scale(.97); }

/* Glassmorphism: light-transmitting surfaces over a calm, readable gradient. */
.workspace[data-theme='glass'] {
  --line:rgba(255,255,255,.48);
  --muted:#687a73;
  background:
    radial-gradient(circle at 16% 14%,rgba(166,224,199,.78),transparent 30%),
    radial-gradient(circle at 88% 12%,rgba(192,207,246,.8),transparent 29%),
    radial-gradient(circle at 74% 88%,rgba(242,190,211,.58),transparent 32%),
    linear-gradient(135deg,#dcefe7 0%,#edf0f8 48%,#f4e4e9 100%);
}
.workspace[data-theme='glass'] .app-rail { color:#dbe9e2; background:rgba(20,54,43,.86); box-shadow:8px 0 30px rgba(37,67,58,.1); backdrop-filter:blur(22px) saturate(130%); -webkit-backdrop-filter:blur(22px) saturate(130%); }
.workspace[data-theme='glass'] .conversation-pane,
.workspace[data-theme='glass'] .inspector { border-color:rgba(255,255,255,.46); background:rgba(250,253,251,.62); box-shadow:12px 0 34px rgba(52,78,70,.07); backdrop-filter:blur(24px) saturate(145%); -webkit-backdrop-filter:blur(24px) saturate(145%); }
.workspace[data-theme='glass'] .chat-stage { background:rgba(247,251,249,.3); }
.workspace[data-theme='glass'] .chat-header,
.workspace[data-theme='glass'] .composer,
.workspace[data-theme='glass'] .inspector > header { border-color:rgba(255,255,255,.52); background:rgba(255,255,255,.48); backdrop-filter:blur(20px) saturate(135%); -webkit-backdrop-filter:blur(20px) saturate(135%); }
.workspace[data-theme='glass'] .search-field { border-color:rgba(255,255,255,.72); background:rgba(255,255,255,.42); box-shadow:inset 0 1px 0 rgba(255,255,255,.5),0 8px 22px rgba(52,77,69,.05); }
.workspace[data-theme='glass'] .search-field:focus-within { border-color:rgba(86,137,106,.46); background:rgba(255,255,255,.7); box-shadow:0 0 0 3px rgba(102,155,121,.1); }
.workspace[data-theme='glass'] .section-tabs button:hover,
.workspace[data-theme='glass'] .session-row:hover,
.workspace[data-theme='glass'] .icon-button:hover { background:rgba(255,255,255,.38); }
.workspace[data-theme='glass'] .section-tabs button.active,
.workspace[data-theme='glass'] .session-row.active,
.workspace[data-theme='glass'] .icon-button.selected { background:rgba(225,244,222,.68); box-shadow:inset 0 0 0 1px rgba(255,255,255,.55),0 8px 20px rgba(54,87,67,.06); }
.workspace[data-theme='glass'] .bubble { border-color:rgba(255,255,255,.72); background:rgba(255,255,255,.78); box-shadow:0 8px 24px rgba(43,68,59,.07),inset 0 1px 0 rgba(255,255,255,.82); }
.workspace[data-theme='glass'] .self .bubble { border-color:rgba(222,247,210,.76); background:rgba(218,241,205,.72); }
.workspace[data-theme='glass'] .theme-menu,
.workspace[data-theme='glass'] .context-menu { border-color:rgba(255,255,255,.68); background:rgba(248,252,250,.76); box-shadow:0 20px 50px rgba(42,68,60,.16),inset 0 1px 0 rgba(255,255,255,.8); backdrop-filter:blur(24px) saturate(145%); -webkit-backdrop-filter:blur(24px) saturate(145%); }
.workspace[data-theme='glass'] .theme-option:hover { background:rgba(255,255,255,.46); }
.workspace[data-theme='glass'] .theme-option.active { background:rgba(221,242,215,.64); }
.workspace[data-theme='glass'] .composer-footer kbd { border-color:rgba(255,255,255,.7); background:rgba(255,255,255,.42); }
.workspace[data-theme='glass'] .messages,
.workspace[data-theme='glass'] .session-list,
.workspace[data-theme='glass'] .member-list { scrollbar-width:thin; scrollbar-color:rgba(63,96,83,.42) rgba(255,255,255,.12); }
.workspace[data-theme='glass'] .messages::-webkit-scrollbar,
.workspace[data-theme='glass'] .session-list::-webkit-scrollbar,
.workspace[data-theme='glass'] .member-list::-webkit-scrollbar { width:10px; height:10px; }
.workspace[data-theme='glass'] .messages::-webkit-scrollbar-track,
.workspace[data-theme='glass'] .session-list::-webkit-scrollbar-track,
.workspace[data-theme='glass'] .member-list::-webkit-scrollbar-track { margin:5px; border-radius:10px; background:rgba(255,255,255,.12); }
.workspace[data-theme='glass'] .messages::-webkit-scrollbar-thumb,
.workspace[data-theme='glass'] .session-list::-webkit-scrollbar-thumb,
.workspace[data-theme='glass'] .member-list::-webkit-scrollbar-thumb { border:3px solid transparent; border-radius:10px; background:rgba(55,91,77,.4); background-clip:padding-box; }
.workspace[data-theme='glass'] .messages::-webkit-scrollbar-thumb:hover,
.workspace[data-theme='glass'] .session-list::-webkit-scrollbar-thumb:hover,
.workspace[data-theme='glass'] .member-list::-webkit-scrollbar-thumb:hover { background:rgba(45,80,66,.58); background-clip:padding-box; }

/* Neumorphism: one soft surface, with depth reserved for controls and bubbles. */
.workspace[data-theme='neumorphic'] { --neu-surface:#e7ece7; --neu-dark:#c5ccc6; --neu-light:#fff; --line:transparent; --muted:#748178; background:var(--neu-surface); }
.workspace[data-theme='neumorphic'] .app-rail,
.workspace[data-theme='neumorphic'] .conversation-pane,
.workspace[data-theme='neumorphic'] .chat-stage,
.workspace[data-theme='neumorphic'] .chat-header,
.workspace[data-theme='neumorphic'] .composer,
.workspace[data-theme='neumorphic'] .inspector,
.workspace[data-theme='neumorphic'] .inspector > header { border:0; background:var(--neu-surface); }
.workspace[data-theme='neumorphic'] .app-rail { color:#597065; box-shadow:inset -1px 0 rgba(177,188,179,.36); }
.workspace[data-theme='neumorphic'] .rail-nav button,
.workspace[data-theme='neumorphic'] .rail-bottom button { color:#82968b; }
.workspace[data-theme='neumorphic'] .conversation-pane { box-shadow:inset -1px 0 rgba(177,188,179,.34); }
.workspace[data-theme='neumorphic'] .inspector { box-shadow:inset 1px 0 rgba(177,188,179,.34); }
.workspace[data-theme='neumorphic'] .chat-header { box-shadow:inset 0 -1px rgba(177,188,179,.3); }
.workspace[data-theme='neumorphic'] .composer { box-shadow:inset 0 1px rgba(177,188,179,.3); }
.workspace[data-theme='neumorphic'] .rail-logo,
.workspace[data-theme='neumorphic'] .rail-nav button.active,
.workspace[data-theme='neumorphic'] .icon-button.selected,
.workspace[data-theme='neumorphic'] .send-button { color:#315d43; background:var(--neu-surface); box-shadow:5px 5px 11px var(--neu-dark),-5px -5px 11px var(--neu-light); }
.workspace[data-theme='neumorphic'] .rail-logo span { background:#315d43; }
.workspace[data-theme='neumorphic'] .rail-nav button:hover,
.workspace[data-theme='neumorphic'] .rail-bottom button:hover,
.workspace[data-theme='neumorphic'] .icon-button:hover,
.workspace[data-theme='neumorphic'] .composer-tools button:hover { color:#315d43; background:var(--neu-surface); box-shadow:3px 3px 7px var(--neu-dark),-3px -3px 7px var(--neu-light); }
.workspace[data-theme='neumorphic'] .rail-nav button.active:active,
.workspace[data-theme='neumorphic'] .icon-button.selected:active,
.workspace[data-theme='neumorphic'] .send-button:active { box-shadow:inset 3px 3px 6px var(--neu-dark),inset -3px -3px 6px var(--neu-light); transform:none; }
.workspace[data-theme='neumorphic'] .search-field { border:0; background:var(--neu-surface); box-shadow:inset 4px 4px 8px var(--neu-dark),inset -4px -4px 8px var(--neu-light); }
.workspace[data-theme='neumorphic'] .search-field:focus-within { border:0; background:var(--neu-surface); box-shadow:inset 3px 3px 7px var(--neu-dark),inset -3px -3px 7px var(--neu-light),0 0 0 2px rgba(66,112,79,.12); }
.workspace[data-theme='neumorphic'] .section-tabs button:hover { background:rgba(255,255,255,.22); }
.workspace[data-theme='neumorphic'] .section-tabs button.active { color:#315d43; background:var(--neu-surface); box-shadow:inset 3px 3px 6px var(--neu-dark),inset -3px -3px 6px var(--neu-light); }
.workspace[data-theme='neumorphic'] .session-row:hover { background:rgba(255,255,255,.2); }
.workspace[data-theme='neumorphic'] .session-row.active { background:var(--neu-surface); box-shadow:inset 4px 4px 8px var(--neu-dark),inset -4px -4px 8px var(--neu-light); }
.workspace[data-theme='neumorphic'] .avatar-wrap > i { border-color:var(--neu-surface); }
.workspace[data-theme='neumorphic'] .bubble { padding:11px 15px; border:0; border-radius:18px 18px 18px 6px; background:var(--neu-surface); box-shadow:4px 4px 9px rgba(180,190,182,.72),-4px -4px 9px rgba(255,255,255,.78); }
.workspace[data-theme='neumorphic'] .self .bubble { border:0; border-radius:18px 18px 6px 18px; color:#284832; background:#dce9d8; box-shadow:4px 4px 9px rgba(177,188,179,.68),-3px -3px 8px rgba(255,255,255,.7); }
.workspace[data-theme='neumorphic'] .file-message > .el-icon:first-child { background:transparent; }
.workspace[data-theme='neumorphic'] .composer textarea { min-height:64px; padding:9px 12px; border-radius:12px; background:var(--neu-surface); box-shadow:inset 3px 3px 7px var(--neu-dark),inset -3px -3px 7px var(--neu-light); resize:none; }
.workspace[data-theme='neumorphic'] .send-button { color:#315d43; }
.workspace[data-theme='neumorphic'] .send-button:hover:not(:disabled) { color:#234a33; background:var(--neu-surface); transform:translateY(-1px); }
.workspace[data-theme='neumorphic'] .send-button:disabled { color:#99a49c; background:var(--neu-surface); box-shadow:inset 2px 2px 5px var(--neu-dark),inset -2px -2px 5px var(--neu-light); }
.workspace[data-theme='neumorphic'] .composer-footer kbd { border:0; background:var(--neu-surface); box-shadow:1px 1px 3px var(--neu-dark),-1px -1px 3px var(--neu-light); }
.workspace[data-theme='neumorphic'] .theme-menu,
.workspace[data-theme='neumorphic'] .context-menu { border:0; background:var(--neu-surface); box-shadow:9px 9px 20px var(--neu-dark),-9px -9px 20px var(--neu-light); }
.workspace[data-theme='neumorphic'] .theme-option:hover { background:rgba(255,255,255,.22); }
.workspace[data-theme='neumorphic'] .theme-option.active { background:var(--neu-surface); box-shadow:inset 3px 3px 7px var(--neu-dark),inset -3px -3px 7px var(--neu-light); }
.workspace[data-theme='neumorphic'] .detail-section,
.workspace[data-theme='neumorphic'] .inspector-profile,
.workspace[data-theme='neumorphic'] .user-result { border-color:rgba(177,188,179,.32); }

:global(html[data-chat-theme='glass'] .el-overlay) { background:rgba(30,48,43,.24); backdrop-filter:blur(5px); -webkit-backdrop-filter:blur(5px); }
:global(html[data-chat-theme='glass'] .el-dialog),
:global(html[data-chat-theme='glass'] .el-popper.is-light) { border-color:rgba(255,255,255,.68); background:rgba(248,252,250,.8); box-shadow:0 24px 60px rgba(31,55,47,.18); backdrop-filter:blur(24px) saturate(145%); -webkit-backdrop-filter:blur(24px) saturate(145%); }
:global(html[data-chat-theme='glass'] .el-dialog .el-input__wrapper),
:global(html[data-chat-theme='glass'] .el-dialog .el-textarea__inner),
:global(html[data-chat-theme='glass'] .el-dialog .el-select__wrapper) { background:rgba(255,255,255,.45); }
:global(html[data-chat-theme='neumorphic'] .el-overlay) { background:rgba(75,87,79,.2); }
:global(html[data-chat-theme='neumorphic'] .el-dialog),
:global(html[data-chat-theme='neumorphic'] .el-popper.is-light) { border:0; background:#e7ece7; box-shadow:12px 12px 28px #bdc5be,-12px -12px 28px rgba(255,255,255,.85); }
:global(html[data-chat-theme='neumorphic'] .el-dialog .el-input__wrapper),
:global(html[data-chat-theme='neumorphic'] .el-dialog .el-textarea__inner),
:global(html[data-chat-theme='neumorphic'] .el-dialog .el-select__wrapper) { border:0; background:#e7ece7; box-shadow:inset 3px 3px 7px #c5ccc6,inset -3px -3px 7px #fff; }

@media (max-width: 1050px) { .workspace { grid-template-columns:58px 274px minmax(0,1fr); }.inspector { position:absolute; z-index:20; top:0; right:0; bottom:0; box-shadow:-8px 0 24px rgba(24,48,31,.08); }.chat-header { padding:0 22px; }.messages { padding-left:30px; padding-right:30px; }.composer { padding-left:30px; padding-right:30px; } }
@media (max-width: 720px) { body { overflow:auto; }.workspace { grid-template-columns:54px minmax(0,1fr); }.conversation-pane { grid-column:2; }.app-rail { padding:12px 0; }.rail-nav { margin-top:27px; }.rail-nav button,.rail-bottom button { width:37px; height:37px; }.chat-stage { display:none; }.workspace:has(.chat-stage .chat-header) .conversation-pane { display:none; }.workspace:has(.chat-stage .chat-header) .chat-stage { display:flex; grid-column:2; }.chat-header { min-height:64px; padding:0 16px; }.mobile-back { display:grid; place-items:center; width:30px; height:30px; margin-right:6px; padding:0; border:0; border-radius:7px; color:#40684a; cursor:pointer; background:transparent; font-size:20px; }.messages { padding:20px 15px; }.composer { padding:10px 15px 13px; }.message-body { max-width:82%; }.composer-footer > span { display:none; }.inspector { width:min(286px, calc(100vw - 54px)); }.section-tabs { padding-bottom:12px; }.pane-head { min-height:67px; }.two-fields { grid-template-columns:1fr; gap:0; } }

/* Web experience refinements */
.workspace { height:100svh; }
.chat-stage { position:relative; }
.message-area { position:relative; min-height:0; flex:1; overflow:hidden; }
.message-area .messages { width:100%; height:100%; }
.connection-banner { display:flex; align-items:center; gap:10px; min-height:42px; padding:7px clamp(22px,5vw,78px); color:#665b35; background:#fff8dd; box-shadow:inset 0 -1px rgba(156,137,73,.12); }
.connection-banner > div { display:flex; min-width:0; flex-wrap:wrap; align-items:baseline; gap:4px 8px; }
.connection-banner strong { font-size:11px; }
.connection-banner span { color:#857a56; font-size:10px; }
.connection-pulse { width:7px; height:7px; flex:0 0 auto; border-radius:50%; background:#d1a843; box-shadow:0 0 0 4px rgba(209,168,67,.13); animation:connection-pulse 1.5s ease-in-out infinite; }
@keyframes connection-pulse { 50% { opacity:.46; transform:scale(.82); } }
.message-stack { gap:0; }
.message-entry { content-visibility:auto; contain-intrinsic-size:68px; }
.date-divider { display:flex; align-items:center; gap:12px; margin:8px 0 22px; color:#8c9890; font-size:10px; font-weight:700; }
.date-divider::before,.date-divider::after { height:1px; flex:1; background:rgba(126,142,132,.15); content:''; }
.date-divider span { flex:0 0 auto; }
.message-row { min-height:32px; }
.message-row.self { flex-direction:row; justify-content:flex-end; }
.message-row.self .message-body { text-align:left; }
.message-entry + .message-entry .message-row.group-start { margin-top:18px; }
.message-row.grouped { margin-top:7px; }
.avatar-spacer { width:32px; height:1px; flex:0 0 32px; }
.message-meta time,.row-top time { font-variant-numeric:tabular-nums; }
.image-preview-button { display:block; max-width:100%; padding:0; overflow:hidden; border:0; border-radius:9px; cursor:zoom-in; background:transparent; }
.image-message { width:auto; height:auto; }
.jump-latest { position:absolute; z-index:12; right:clamp(22px,5vw,78px); bottom:18px; display:inline-flex; align-items:center; gap:6px; min-height:34px; padding:0 12px; border:1px solid #dce5da; border-radius:18px; color:#365d40; cursor:pointer; background:rgba(255,255,255,.94); box-shadow:0 8px 24px rgba(33,58,41,.12); font-size:10px; font-weight:800; animation:jump-latest-in .18s ease-out; }
.jump-latest:hover { color:#214b2d; transform:translateY(-1px); }
@keyframes jump-latest-in { from { opacity:0; transform:translateY(6px); } }
.messages,.session-list,.member-list { overscroll-behavior:contain; scrollbar-gutter:stable; }
.session-row { content-visibility:auto; contain-intrinsic-size:62px; }
.composer-tools button { width:32px; height:32px; }
.composer textarea { overflow-y:auto; resize:none; }
.send-button { min-height:34px; }
.workspace :where(button,a,input,textarea):focus-visible { outline:2px solid #4e7d57; outline-offset:2px; }
.workspace[data-theme='glass'] .connection-banner { color:#584f31; background:rgba(255,249,222,.65); backdrop-filter:blur(16px); -webkit-backdrop-filter:blur(16px); }
.workspace[data-theme='glass'] .jump-latest { border-color:rgba(255,255,255,.75); background:rgba(255,255,255,.68); backdrop-filter:blur(18px); -webkit-backdrop-filter:blur(18px); }
.workspace[data-theme='neumorphic'] .connection-banner { color:#665b35; background:var(--neu-surface); box-shadow:inset 0 -1px rgba(177,188,179,.34); }
.workspace[data-theme='neumorphic'] .jump-latest { border:0; background:var(--neu-surface); box-shadow:5px 5px 11px var(--neu-dark),-5px -5px 11px var(--neu-light); }
.workspace[data-theme='neumorphic'] .message-row.grouped .bubble { box-shadow:2px 2px 6px rgba(180,190,182,.56),-2px -2px 6px rgba(255,255,255,.66); }
@supports not ((backdrop-filter:blur(2px)) or (-webkit-backdrop-filter:blur(2px))) {
  .workspace[data-theme='glass'] .conversation-pane,
  .workspace[data-theme='glass'] .inspector,
  .workspace[data-theme='glass'] .chat-header,
  .workspace[data-theme='glass'] .composer,
  .workspace[data-theme='glass'] .theme-menu { background:rgba(247,251,249,.94); }
}
@media (prefers-reduced-motion:reduce) {
  .workspace *,
  .workspace *::before,
  .workspace *::after { scroll-behavior:auto!important; animation-duration:.01ms!important; animation-iteration-count:1!important; transition-duration:.01ms!important; }
}
@media (max-width:720px) {
  .app-rail { padding-top:calc(12px + env(safe-area-inset-top)); padding-bottom:calc(12px + env(safe-area-inset-bottom)); }
  .rail-nav button,.rail-bottom button { width:42px; height:42px; }
  .mobile-back { width:38px; height:38px; }
  .composer { padding-bottom:calc(13px + env(safe-area-inset-bottom)); }
  .jump-latest { right:15px; bottom:15px; }
}
</style>
