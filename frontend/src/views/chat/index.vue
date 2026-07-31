<template>
  <div class="workspace" @click="closeMenus">
    <aside class="app-rail" aria-label="主导航">
      <button class="rail-logo" aria-label="ChatRoom 首页"><span></span><span></span><span></span></button>
      <nav class="rail-nav">
        <el-tooltip content="会话" placement="right"><button :class="{ active: activeSection === 'chats' }" @click.stop="activeSection = 'chats'"><el-icon><ChatDotRound /></el-icon><b v-if="totalUnread">{{ totalUnread > 99 ? '99+' : totalUnread }}</b></button></el-tooltip>
        <el-tooltip content="联系人" placement="right"><button :class="{ active: activeSection === 'contacts' }" @click.stop="activeSection = 'contacts'"><el-icon><User /></el-icon></button></el-tooltip>
        <el-tooltip content="群组" placement="right"><button :class="{ active: activeSection === 'groups' }" @click.stop="activeSection = 'groups'"><el-icon><UserFilled /></el-icon></button></el-tooltip>
      </nav>
      <div class="rail-bottom">
        <el-tooltip content="个人资料" placement="right"><button @click.stop="openProfile"><el-avatar :size="30" :src="assetUrl(userStore.userInfo?.avatar)">{{ initial(userStore.userInfo?.nickname) }}</el-avatar></button></el-tooltip>
      </div>
    </aside>

    <aside class="conversation-pane">
      <header class="pane-head">
        <div class="identity">
          <span class="connection-dot" :class="{ connected }"></span>
          <div><strong>{{ userStore.userInfo?.nickname || 'ChatRoom' }}</strong><small>{{ connected ? '已连接' : '正在连接…' }}</small></div>
        </div>
        <el-dropdown trigger="click" @command="handleAccountCommand">
          <button class="icon-button" aria-label="账户操作" @click.stop><el-icon><MoreFilled /></el-icon></button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="profile">编辑个人资料</el-dropdown-item>
              <el-dropdown-item command="logout" divided>退出登录</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </header>

      <div class="section-tabs" aria-label="内容分类">
        <button :class="{ active: activeSection === 'chats' }" @click="activeSection = 'chats'">消息</button>
        <button :class="{ active: activeSection === 'contacts' }" @click="activeSection = 'contacts'">联系人</button>
        <button :class="{ active: activeSection === 'groups' }" @click="activeSection = 'groups'">群组</button>
      </div>

      <div class="search-field">
        <el-icon><Search /></el-icon>
        <input v-model.trim="searchText" :placeholder="sectionSearchPlaceholder" aria-label="搜索当前列表">
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

      <div v-if="contextMenu.visible" class="context-menu" :style="{ left: contextMenu.left + 'px', top: contextMenu.top + 'px' }" @click.stop>
        <button @click="contextMenu.visible = false">关闭菜单</button>
        <button v-if="contextMenu.item?.type === 'user'" class="danger" @click="removeFriend(contextMenu.item)">删除联系人</button>
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

        <section ref="messageListRef" class="messages" @scroll="handleMessageScroll">
          <div v-if="historyLoading" class="history-state"><el-icon class="is-loading"><Loading /></el-icon> 正在加载消息</div>
          <template v-else>
            <div v-if="!currentMessages.length" class="conversation-empty">
              <div class="empty-orb"><el-icon><ChatLineRound /></el-icon></div>
              <strong>从一句问候开始</strong>
              <p>新消息会实时出现在这里。</p>
            </div>
            <transition-group v-else name="message" tag="div" class="message-stack">
              <article v-for="message in currentMessages" :key="message.msg_id" class="message-row" :class="{ self: isSelf(message), revoked: message.status === 2 }">
                <el-avatar v-if="!isSelf(message)" :size="32" :src="messageAvatar(message)">{{ initial(messageSender(message)) }}</el-avatar>
                <div class="message-body">
                  <div v-if="chatStore.currentChat.type === 'group' && !isSelf(message)" class="sender-name">{{ messageSender(message) }}</div>
                  <div class="bubble">
                    <template v-if="message.status === 2"><span class="recalled">此消息已撤回</span></template>
                    <template v-else-if="message.content_type === 'image'"><img :src="assetUrl(message.content)" alt="发送的图片" class="image-message" @click="previewImage(message.content)"></template>
                    <template v-else-if="message.content_type === 'file'"><a :href="assetUrl(message.content)" class="file-message" target="_blank" :download="fileName(message)" @click.prevent="downloadAttachment(message)"><el-icon><Document /></el-icon><span><strong>{{ fileName(message) }}</strong><small>点击下载文件</small></span><el-icon><Download /></el-icon></a></template>
                    <template v-else>{{ message.content }}</template>
                  </div>
                  <div class="message-meta"><time>{{ fullTime(message.timestamp) }}</time><span v-if="isSelf(message) && message.local_status === 'sending'">发送中</span><span v-else-if="isSelf(message) && message.local_status === 'failed'" class="failed">未发送</span><span v-else-if="isSelf(message)">已发送</span><button v-if="canRevoke(message)" title="撤回消息" @click="revoke(message)"><el-icon><RefreshLeft /></el-icon>撤回</button></div>
                </div>
                <el-avatar v-if="isSelf(message)" :size="32" :src="assetUrl(userStore.userInfo?.avatar)">{{ initial(userStore.userInfo?.nickname) }}</el-avatar>
              </article>
            </transition-group>
          </template>
        </section>

        <div v-if="typingText" class="typing-status"><span></span><span></span><span></span>{{ typingText }}</div>
        <footer class="composer">
          <div class="composer-tools">
            <input ref="fileInputRef" class="file-input" type="file" accept="image/*,.pdf,.doc,.docx,.txt,.zip,.rar,.mp4,.webm" @change="uploadAttachment">
            <button :disabled="uploading" title="发送文件" @click="fileInputRef?.click()"><el-icon><Paperclip /></el-icon></button>
            <span>{{ uploading ? '正在上传文件…' : '支持图片、文档与压缩包，最大 50 MB' }}</span>
          </div>
          <textarea ref="composerRef" v-model="draft" :disabled="uploading" placeholder="输入消息，Ctrl + Enter 发送" @input="handleDraftInput" @keydown="handleComposerKeydown"></textarea>
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
        <div class="detail-section"><div class="detail-title"><span>成员</span><button v-if="isGroupOwner" @click="showInviteDialog = true"><el-icon><Plus /></el-icon>邀请</button></div><div class="member-list"><div v-for="member in groupMembers" :key="member.id" class="member-row"><el-avatar :size="30" :src="assetUrl(member.avatar)">{{ initial(member.nickname) }}</el-avatar><span>{{ member.nickname || member.username }}</span><em v-if="member.role === 2">群主</em><em v-else-if="member.role === 1">管理员</em><button v-if="isGroupOwner && member.role !== 2" title="移除成员" @click="removeMemberFromGroup(member)"><el-icon><Close /></el-icon></button></div></div></div>
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
import { downloadFile, markRead, revokeMessage, resolveStoredFileUrl, uploadFile } from '../../api/message'
import { connected, connect, disconnect, sendChatMessage, sendReadReceipt, sendTyping, subscribe } from '../../websocket'

const router = useRouter()
const userStore = useUserStore()
const chatStore = useChatStore()

const activeSection = ref('chats')
const searchText = ref('')
const draft = ref('')
const historyLoading = ref(false)
const messageListRef = ref()
const composerRef = ref()
const fileInputRef = ref()
const uploading = ref(false)
const inspectorOpen = ref(false)
const groupInfo = ref(null)
const groupMembers = ref([])
const typingUser = ref('')
const previewingImage = ref('')
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
let typingTimer
let lastTypingAt = 0
let unsubscribeCallbacks = []

const sectionTitle = computed(() => ({ chats: '最近会话', contacts: '联系人', groups: '我的群组' })[activeSection.value])
const sectionSearchPlaceholder = computed(() => ({ chats: '搜索会话', contacts: '搜索联系人', groups: '搜索群组' })[activeSection.value])
const emptyListCopy = computed(() => ({ chats: '还没有可显示的会话', contacts: '还没有联系人', groups: '还没有加入群组' })[activeSection.value])
const totalUnread = computed(() => Object.values(chatStore.unreadCounts).reduce((sum, count) => sum + (count || 0), 0))
const currentMessages = computed(() => {
  const current = chatStore.currentChat
  return current ? chatStore.chatMessages[chatStore.chatKey(current.type, current.id)] || [] : []
})
const currentOnline = computed(() => Boolean(chatStore.currentChat && chatStore.onlineUsers[chatStore.currentChat.id]))
const chatSubtitle = computed(() => {
  const current = chatStore.currentChat
  if (!current) return ''
  if (current.type === 'user') return currentOnline.value ? '在线 · 实时连接中' : '离线'
  return (groupMembers.value.length || current.member_count || '—') + ' 位成员'
})
const typingText = computed(() => typingUser.value ? typingUser.value + ' 正在输入…' : '')
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
  if (message.status === 2) return '一条消息已撤回'
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

function shortTime(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp)
  const today = new Date()
  if (date.toDateString() === today.toDateString()) return String(date.getHours()).padStart(2, '0') + ':' + String(date.getMinutes()).padStart(2, '0')
  return String(date.getMonth() + 1).padStart(2, '0') + '/' + String(date.getDate()).padStart(2, '0')
}

function fullTime(timestamp) {
  if (!timestamp) return ''
  const date = new Date(timestamp)
  const today = new Date()
  if (date.toDateString() === today.toDateString()) return String(date.getHours()).padStart(2, '0') + ':' + String(date.getMinutes()).padStart(2, '0')
  return (date.getMonth() + 1) + '月' + date.getDate() + '日 ' + String(date.getHours()).padStart(2, '0') + ':' + String(date.getMinutes()).padStart(2, '0')
}

async function selectItem(item) {
  closeMenus()
  const chat = { ...item, name: itemName(item) }
  chatStore.setCurrentChat(chat)
  activeSection.value = item.type === 'group' ? 'groups' : 'chats'
  inspectorOpen.value = false
  typingUser.value = ''
  historyLoading.value = true
  try {
    await chatStore.fetchHistory(item.id, item.type)
    if (item.type === 'group') await loadGroupDetails(item.id)
    await nextTick()
    scrollToBottom()
    markCurrentRead()
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

function scrollToBottom() {
  nextTick(() => {
    if (messageListRef.value) messageListRef.value.scrollTop = messageListRef.value.scrollHeight
  })
}

function handleMessageScroll() {
  const node = messageListRef.value
  if (node && node.scrollTop + node.clientHeight >= node.scrollHeight - 60) markCurrentRead()
}

function markCurrentRead() {
  const current = chatStore.currentChat
  const last = currentMessages.value.at(-1)
  if (!current || !last?.msg_id || isSelf(last)) return
  markRead({ target_id: current.id, target_type: current.type, last_msg_id: last.msg_id }).catch(() => {})
  sendReadReceipt(current.id, current.type, last.msg_id)
}

function isSelf(message) {
  return message.from_id === userStore.userInfo?.id || message.from_user_id === userStore.userInfo?.id
}

function canRevoke(message) {
  return isSelf(message) && message.status !== 2 && Boolean(message.msg_id)
}

function queueMessage(contentType, content) {
  const current = chatStore.currentChat
  if (!current) return
  const msgId = sendChatMessage({ toId: current.id, toType: current.type, contentType, content })
  if (!msgId) {
    ElMessage.warning('实时连接尚未建立，请稍后重试')
    return
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
}

function sendText() {
  const content = draft.value.trim()
  if (!content || uploading.value) return
  queueMessage('text', content)
  draft.value = ''
  composerRef.value?.focus()
}

function handleComposerKeydown(event) {
  if ((event.ctrlKey || event.metaKey) && event.key === 'Enter') {
    event.preventDefault()
    sendText()
  }
}

function handleDraftInput() {
  const current = chatStore.currentChat
  if (!current || !draft.value.trim() || !connected.value) return
  const now = Date.now()
  if (now - lastTypingAt > 2500) {
    sendTyping(current.id, current.type)
    lastTypingAt = now
  }
  window.clearTimeout(typingTimer)
  typingTimer = window.setTimeout(() => { lastTypingAt = 0 }, 3000)
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

async function revoke(message) {
  try {
    await ElMessageBox.confirm('撤回后，所有成员将看到撤回提示。', '撤回此消息？', { confirmButtonText: '撤回', cancelButtonText: '取消', type: 'warning' })
    await revokeMessage(message.msg_id)
    chatStore.updateMessageStatus(message.msg_id, 2)
    ElMessage.success('消息已撤回')
  } catch (error) {
    if (error !== 'cancel' && error !== 'close') throw error
  }
}

function previewImage(url) {
  previewingImage.value = assetUrl(url)
}

function openContextMenu(item, event) {
  contextMenu.item = item
  contextMenu.left = event.clientX
  contextMenu.top = event.clientY
  contextMenu.visible = true
}

function closeMenus() {
  contextMenu.visible = false
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

watch(() => chatStore.currentChat?.id, () => { inviteIds.value = [] })

onMounted(async () => {
  try {
    await userStore.fetchProfile()
    unsubscribeCallbacks = [
      subscribe('message', (message) => {
        const key = chatStore.addMessage(message)
        if (key === (chatStore.currentChat && chatStore.chatKey(chatStore.currentChat.type, chatStore.currentChat.id))) {
          scrollToBottom()
          markCurrentRead()
        }
      }),
      subscribe('ack', (ack) => {
        Object.values(chatStore.chatMessages).forEach((list) => {
          const message = list.find((item) => item.msg_id === ack.msg_id)
          if (message) message.local_status = ack.status === 'sent' ? 'sent' : message.local_status
        })
      }),
      subscribe('onlineStatus', (status) => chatStore.setOnlineStatus(status.user_id, status.online)),
      subscribe('typing', (message) => {
        const data = message.data || {}
        const current = chatStore.currentChat
        const matches = current && ((current.type === 'group' && current.id === message.to_id) || (current.type === 'user' && current.id === data.user_id))
        if (matches && data.user_id !== userStore.userInfo?.id) {
          typingUser.value = data.username || '对方'
          window.clearTimeout(typingTimer)
          typingTimer = window.setTimeout(() => { typingUser.value = '' }, 2800)
        }
      }),
      subscribe('sessionReplaced', () => {
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
  window.clearTimeout(typingTimer)
  disconnect()
})
</script>

<style scoped>
.workspace { --forest:#19362c; --forest-soft:#28543d; --mint:#dcefd0; --lime:#bfe879; --ink:#1d2821; --muted:#7a887e; --line:#e6ebe4; display:grid; grid-template-columns:64px 306px minmax(0,1fr); min-height:100svh; overflow:hidden; color:var(--ink); background:#f7f8f5; }
.app-rail { display:flex; flex-direction:column; align-items:center; padding:16px 0; color:#b6c8b9; background:var(--forest); }.rail-logo { display:flex; align-items:flex-end; justify-content:center; gap:3px; width:34px; height:34px; padding:0 7px 7px; border:0; border-radius:10px; cursor:pointer; background:var(--mint); }.rail-logo span { width:4px; border-radius:10px; background:var(--forest); }.rail-logo span:nth-child(1){height:9px}.rail-logo span:nth-child(2){height:16px}.rail-logo span:nth-child(3){height:22px}.rail-nav { display:grid; gap:10px; margin-top:43px; }.rail-nav button,.rail-bottom button { position:relative; display:grid; place-items:center; width:42px; height:42px; border:0; border-radius:11px; color:#b6c8b9; cursor:pointer; background:transparent; transition:color .18s,background .18s,transform .18s; }.rail-nav button:hover,.rail-bottom button:hover { color:#fff; background:rgba(255,255,255,.09); }.rail-nav button.active { color:var(--forest); background:var(--mint); }.rail-nav .el-icon { font-size:20px; }.rail-nav b { position:absolute; top:-4px; right:-7px; min-width:17px; padding:2px 4px; border:2px solid var(--forest); border-radius:10px; color:#fff; background:#e96061; font-size:9px; line-height:1; }.rail-bottom { margin-top:auto; }.rail-bottom button { overflow:hidden; }
.conversation-pane { position:relative; display:flex; flex-direction:column; min-width:0; border-right:1px solid var(--line); background:#fff; }.pane-head { display:flex; align-items:center; justify-content:space-between; min-height:76px; padding:0 18px 0 20px; }.identity { display:flex; align-items:center; gap:10px; }.identity strong { display:block; font-size:13px; line-height:1.3; }.identity small { display:block; margin-top:2px; color:var(--muted); font-size:11px; }.connection-dot { width:8px; height:8px; border-radius:50%; background:#b9c2ba; }.connection-dot.connected { background:#65af71; box-shadow:0 0 0 4px rgba(101,175,113,.11); }.icon-button { display:grid; place-items:center; width:32px; height:32px; border:0; border-radius:8px; color:#7c897f; cursor:pointer; background:transparent; transition:background .18s,color .18s; }.icon-button:hover,.icon-button.selected { color:#244630; background:#edf3e9; }.icon-button .el-icon { font-size:18px; }.section-tabs { display:flex; gap:4px; padding:0 15px 14px; }.section-tabs button { flex:1; height:32px; border:0; border-radius:7px; color:#87938a; cursor:pointer; background:transparent; font-size:12px; font-weight:700; }.section-tabs button:hover { color:#355b40; background:#f4f7f2; }.section-tabs button.active { color:#28583c; background:#eaf4e5; }.search-field { display:flex; align-items:center; gap:8px; height:36px; margin:0 17px; padding:0 10px; border:1px solid #e6ece3; border-radius:8px; color:#91a096; background:#f8faf7; transition:border .18s,background .18s; }.search-field:focus-within { border-color:#8ab38b; background:#fff; }.search-field .el-icon { font-size:15px; }.search-field input { width:100%; min-width:0; border:0; outline:0; color:#26332b; background:transparent; font-size:12px; }.search-field input::placeholder { color:#9aa59d; }.search-field button { display:grid; place-items:center; padding:0; border:0; color:#98a39b; cursor:pointer; background:transparent; }.list-heading { display:flex; align-items:center; justify-content:space-between; padding:24px 20px 10px; color:#69776e; font-size:11px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }.small-action { display:inline-flex; align-items:center; gap:3px; padding:3px 0; border:0; color:#397044; cursor:pointer; background:none; font-size:11px; font-weight:800; }.small-action:hover { color:#173f25; }.small-action .el-icon { font-size:13px; }.session-list { flex:1; overflow-y:auto; padding:0 9px 15px; }.session-row { display:flex; align-items:center; gap:10px; width:100%; padding:10px 10px; border:0; border-radius:10px; color:inherit; cursor:pointer; text-align:left; background:transparent; transition:background .16s,transform .16s; }.session-row:hover { background:#f5f8f3; }.session-row.active { background:#eaf4e5; }.avatar-wrap { position:relative; display:inline-grid; flex:0 0 auto; place-items:center; }.avatar-wrap > i { position:absolute; right:-1px; bottom:0; width:10px; height:10px; border:2px solid #fff; border-radius:50%; background:#65af71; }.group-avatar-mark { position:absolute; right:-4px; bottom:-3px; display:grid; place-items:center; width:14px; height:14px; border:1px solid #fff; border-radius:50%; color:#477250; background:#d9ebd4; }.group-avatar-mark .el-icon { font-size:8px; }.row-copy { min-width:0; flex:1; }.row-top,.row-bottom { display:flex; align-items:center; justify-content:space-between; gap:6px; }.row-top strong { overflow:hidden; color:#2a382f; font-size:13px; font-weight:700; text-overflow:ellipsis; white-space:nowrap; }.row-top time { flex:0 0 auto; color:#a0aca2; font-size:10px; }.row-bottom { margin-top:4px; }.row-bottom > span { overflow:hidden; color:#929d94; font-size:11px; text-overflow:ellipsis; white-space:nowrap; }.row-bottom em { display:grid; flex:0 0 auto; place-items:center; min-width:16px; height:16px; padding:0 4px; border-radius:8px; color:#fff; background:#4d9457; font-size:9px; font-style:normal; font-weight:800; }.list-empty { display:grid; place-items:center; align-content:center; min-height:210px; padding:20px; color:#9aa59d; text-align:center; }.list-empty .el-icon { font-size:30px; color:#b7c5b8; }.list-empty p { margin:10px 0; font-size:12px; }.list-empty button { border:0; color:#397044; cursor:pointer; background:none; font-size:12px; font-weight:800; }.context-menu { position:fixed; z-index:50; min-width:145px; padding:5px; border:1px solid #e6ebe4; border-radius:9px; box-shadow:0 14px 30px rgba(23,45,30,.13); background:#fff; }.context-menu button { display:block; width:100%; padding:8px 10px; border:0; border-radius:6px; color:#59675d; cursor:pointer; text-align:left; background:transparent; font-size:12px; }.context-menu button:hover { background:#f4f7f2; }.context-menu .danger { color:#bd4e52; }
.chat-stage { display:flex; flex-direction:column; min-width:0; min-height:0; background:#f8faf7; }.chat-header { display:flex; align-items:center; justify-content:space-between; min-height:76px; padding:0 28px; border-bottom:1px solid var(--line); background:rgba(255,255,255,.74); }.mobile-back { display:none; }.chat-title { display:flex; align-items:center; gap:11px; min-width:0; }.chat-title h1 { overflow:hidden; margin:0; color:#26342b; font-size:15px; letter-spacing:-.02em; text-overflow:ellipsis; white-space:nowrap; }.chat-title p { margin:3px 0 0; color:#8b988f; font-size:11px; }.detail-button { width:35px; height:35px; }.messages { flex:1; min-height:0; overflow-y:auto; padding:28px clamp(22px,5vw,78px); scroll-behavior:smooth; }.message-stack { display:flex; flex-direction:column; gap:22px; max-width:880px; margin:auto; }.message-row { display:flex; align-items:flex-start; gap:9px; }.message-row.self { flex-direction:row-reverse; }.message-body { max-width:min(74%,570px); }.sender-name { margin:0 0 4px 3px; color:#809086; font-size:11px; }.bubble { min-width:48px; padding:10px 13px; border:1px solid #e5ebe3; border-radius:4px 15px 15px 15px; color:#27352c; background:#fff; box-shadow:0 2px 4px rgba(34,55,39,.025); font-size:13px; line-height:1.65; word-break:break-word; }.self .bubble { border-color:#d6e7c9; border-radius:15px 4px 15px 15px; color:#24402b; background:#e0f1d4; }.recalled .bubble { padding:7px 11px; border-color:transparent; color:#9aa49d; background:#eef1ed; font-size:12px; font-style:italic; }.recalled { color:#9aa49d; }.message-meta { display:flex; align-items:center; gap:6px; margin:4px 3px 0; color:#a1aaa3; font-size:10px; }.self .message-meta { justify-content:flex-end; }.message-meta .failed { color:#c65c5e; }.message-meta button { display:inline-flex; align-items:center; gap:2px; padding:0; border:0; color:#9ca69e; cursor:pointer; background:transparent; font-size:10px; }.message-meta button:hover { color:#be595c; }.message-meta .el-icon { font-size:11px; }.image-message { display:block; max-width:min(330px,60vw); max-height:300px; border-radius:9px; cursor:zoom-in; object-fit:contain; }.file-message { display:flex; align-items:center; gap:10px; min-width:190px; color:#315d3a; text-decoration:none; }.file-message > .el-icon:first-child { display:grid; flex:0 0 auto; place-items:center; width:32px; height:32px; border-radius:8px; color:#477d50; background:#d5ebcc; font-size:17px; }.file-message span { min-width:0; flex:1; }.file-message strong,.file-message small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.file-message strong { font-size:12px; }.file-message small { margin-top:1px; color:#7f9581; font-size:10px; }.file-message > .el-icon:last-child { font-size:15px; }.history-state { display:flex; justify-content:center; gap:7px; padding:25px; color:#89968c; font-size:12px; }.conversation-empty { display:grid; place-items:center; align-content:center; min-height:100%; color:#87958b; text-align:center; }.empty-orb { display:grid; place-items:center; width:52px; height:52px; margin-bottom:13px; border-radius:20px; color:#477854; background:#e5f0df; }.empty-orb .el-icon { font-size:24px; }.conversation-empty strong { color:#4c5e51; font-size:14px; }.conversation-empty p { margin:6px 0; font-size:12px; }.typing-status { display:flex; align-items:center; gap:3px; min-height:24px; padding:0 clamp(22px,5vw,78px); color:#849086; background:#fff; font-size:10px; }.typing-status span { width:4px; height:4px; border-radius:50%; background:#6b9f70; animation:dot-bounce 1s infinite ease-in-out; }.typing-status span:nth-child(2){animation-delay:.16s}.typing-status span:nth-child(3){animation-delay:.32s;margin-right:4px}@keyframes dot-bounce{50%{transform:translateY(-3px);opacity:.45}}.composer { padding:13px clamp(22px,5vw,78px) 16px; border-top:1px solid var(--line); background:#fff; }.composer-tools { display:flex; align-items:center; gap:8px; min-height:27px; color:#94a098; font-size:10px; }.composer-tools button { display:grid; place-items:center; width:27px; height:25px; padding:0; border:0; border-radius:6px; color:#617568; cursor:pointer; background:transparent; }.composer-tools button:hover { color:#28583c; background:#eef5ea; }.composer-tools button:disabled { opacity:.5; cursor:not-allowed; }.composer-tools .el-icon { font-size:17px; }.file-input { display:none; }.composer textarea { display:block; width:100%; min-height:56px; max-height:160px; margin:3px 0; padding:5px 0; border:0; outline:0; resize:vertical; color:#27352b; background:transparent; font-size:13px; line-height:1.6; }.composer textarea::placeholder { color:#a7b0a9; }.composer-footer { display:flex; align-items:center; justify-content:space-between; color:#9ba69e; font-size:10px; }.composer-footer kbd { padding:1px 4px; border:1px solid #dbe2da; border-radius:3px; background:#f6f8f5; font-family:inherit; font-size:9px; }.send-button { display:inline-flex; align-items:center; gap:6px; height:30px; padding:0 13px; border:0; border-radius:7px; color:#f6faf4; cursor:pointer; background:#315f3d; font-size:11px; font-weight:800; transition:background .16s,transform .16s; }.send-button:hover:not(:disabled) { background:#1e4a2c; transform:translateY(-1px); }.send-button:disabled { color:#a7b2a8; cursor:not-allowed; background:#e7ebe6; }
.welcome-stage { display:grid; flex:1; place-items:center; align-content:center; padding:35px; color:#728076; text-align:center; }.welcome-art { position:relative; width:104px; height:88px; margin-bottom:20px; }.welcome-art span { position:absolute; display:block; border:2px solid #5d9367; border-radius:16px; background:#e7f3df; }.welcome-art span:nth-child(1) { top:3px; left:7px; width:69px; height:51px; }.welcome-art span:nth-child(2) { right:2px; bottom:1px; width:62px; height:47px; border-color:#8db172; background:#f4faef; }.welcome-art span:nth-child(3) { bottom:11px; left:0; width:18px; height:18px; border:0; border-radius:50%; background:#bcdf91; }.welcome-stage .eyebrow { color:#76a679; }.welcome-stage h1 { max-width:500px; margin:12px 0 8px; color:#33453a; font-size:clamp(24px,3vw,34px); letter-spacing:-.055em; }.welcome-stage > p:last-of-type { max-width:350px; margin:0; font-size:13px; line-height:1.7; }.welcome-stage button { display:inline-flex; align-items:center; gap:5px; margin-top:23px; padding:9px 13px; border:1px solid #cce0c8; border-radius:8px; color:#315f3d; cursor:pointer; background:#fff; font-size:12px; font-weight:800; }.welcome-stage button:hover { background:#eef6ea; }
.inspector { display:flex; flex-direction:column; width:286px; min-height:0; border-left:1px solid var(--line); background:#fff; animation:inspector-in .2s ease-out; }.inspector > header { display:flex; align-items:center; justify-content:space-between; min-height:76px; padding:0 18px; border-bottom:1px solid var(--line); color:#56655a; font-size:12px; font-weight:800; }.inspector-profile { padding:25px 18px 22px; border-bottom:1px solid var(--line); text-align:center; }.inspector-profile h2 { margin:10px 0 3px; color:#314038; font-size:16px; letter-spacing:-.03em; }.inspector-profile p { margin:0; color:#8a978d; font-size:11px; }.detail-section { padding:19px 18px; border-bottom:1px solid #eef1ed; }.detail-label { color:#87958a; font-size:10px; font-weight:800; letter-spacing:.08em; text-transform:uppercase; }.detail-section > p { margin:8px 0 0; color:#506055; font-size:12px; line-height:1.7; }.detail-title { display:flex; align-items:center; justify-content:space-between; color:#65736a; font-size:11px; font-weight:800; }.detail-title button { display:inline-flex; align-items:center; gap:2px; padding:0; border:0; color:#3d7548; cursor:pointer; background:none; font-size:11px; font-weight:800; }.member-list { max-height:220px; margin-top:11px; overflow-y:auto; }.member-row { display:flex; align-items:center; gap:8px; min-height:39px; }.member-row > span { overflow:hidden; flex:1; color:#4d5d52; font-size:12px; text-overflow:ellipsis; white-space:nowrap; }.member-row em { color:#7c9b70; font-size:10px; font-style:normal; }.member-row button { display:grid; place-items:center; padding:3px; border:0; color:#a5aea6; cursor:pointer; background:transparent; }.member-row button:hover { color:#be595c; }.danger-zone { margin-top:auto; padding:16px 18px; }.danger-zone button { width:100%; height:34px; border:1px solid #f0d4d5; border-radius:7px; color:#b95659; cursor:pointer; background:#fffafa; font-size:11px; font-weight:800; }.danger-zone button:hover { background:#fff1f1; }@keyframes inspector-in{from{opacity:0;transform:translateX(12px)}to{opacity:1;transform:translateX(0)}}.message-enter-active{transition:opacity .22s ease,transform .22s ease}.message-enter-from{opacity:0;transform:translateY(8px)}
.dialog-note { margin:0 0 16px; color:#758278; font-size:12px; line-height:1.6; }.dialog-search { display:flex; gap:9px; }.user-results { margin-top:14px; }.user-result { display:flex; align-items:center; gap:10px; padding:10px 0; border-bottom:1px solid #edf1ec; }.user-result > div { min-width:0; flex:1; }.user-result strong,.user-result small { display:block; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.user-result strong { color:#35443a; font-size:13px; }.user-result small { margin-top:2px; color:#91a097; font-size:11px; }.user-result :deep(.el-button) { border-radius:7px; font-size:12px; }.two-fields { display:grid; grid-template-columns:1fr 1fr; gap:12px; }.app-dialog :deep(.el-dialog) { border-radius:13px; }.app-dialog :deep(.el-dialog__header) { margin-right:0; padding:20px 22px 11px; }.app-dialog :deep(.el-dialog__title) { color:#344238; font-size:16px; font-weight:800; }.app-dialog :deep(.el-dialog__body) { padding:13px 22px 17px; }.app-dialog :deep(.el-dialog__footer) { padding:10px 22px 19px; }.app-dialog :deep(.el-form-item) { margin-bottom:14px; }.app-dialog :deep(.el-form-item__label) { padding-bottom:5px; color:#66736a; font-size:12px; font-weight:700; }.app-dialog :deep(.el-input__wrapper),.app-dialog :deep(.el-textarea__inner),.app-dialog :deep(.el-select__wrapper) { border-radius:8px; box-shadow:0 0 0 1px #e0e7df inset; }.app-dialog :deep(.el-button--primary) { border-color:#315f3d; background:#315f3d; }
@media (max-width: 1050px) { .workspace { grid-template-columns:58px 274px minmax(0,1fr); }.inspector { position:absolute; z-index:20; top:0; right:0; bottom:0; box-shadow:-8px 0 24px rgba(24,48,31,.08); }.chat-header { padding:0 22px; }.messages { padding-left:30px; padding-right:30px; }.composer { padding-left:30px; padding-right:30px; } }
@media (max-width: 720px) { body { overflow:auto; }.workspace { grid-template-columns:54px minmax(0,1fr); }.conversation-pane { grid-column:2; }.app-rail { padding:12px 0; }.rail-nav { margin-top:27px; }.rail-nav button,.rail-bottom button { width:37px; height:37px; }.chat-stage { display:none; }.workspace:has(.chat-stage .chat-header) .conversation-pane { display:none; }.workspace:has(.chat-stage .chat-header) .chat-stage { display:flex; grid-column:2; }.chat-header { min-height:64px; padding:0 16px; }.mobile-back { display:grid; place-items:center; width:30px; height:30px; margin-right:6px; padding:0; border:0; border-radius:7px; color:#40684a; cursor:pointer; background:transparent; font-size:20px; }.messages { padding:20px 15px; }.composer { padding:10px 15px 13px; }.message-body { max-width:82%; }.composer-footer > span { display:none; }.inspector { width:min(286px, calc(100vw - 54px)); }.section-tabs { padding-bottom:12px; }.pane-head { min-height:67px; }.two-fields { grid-template-columns:1fr; gap:0; } }
</style>
