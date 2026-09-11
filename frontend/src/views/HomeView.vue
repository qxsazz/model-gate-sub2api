<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-else
            to="/docs"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </router-link>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- New Design: MODEL-GATE -->
  <div v-else class="model-gate-home" :class="{ 'is-dark': isDark }" ref="scrollContainer">
    <!-- Top Navigation Bar -->
    <nav class="top-nav" :class="{ scrolled: isScrolled }">
      <div class="nav-container">
        <div class="nav-left">
          <div class="nav-logo">
            <img class="logo-image" src="/logo.svg" alt="MODEL-GATE" />
            <span class="logo-text">MODEL-GATE</span>
          </div>
        </div>

        <div class="nav-center">
          <a href="#hero" class="nav-link">主页</a>
          <router-link to="/docs" class="nav-link">文档</router-link>
          <a href="#dashboard" class="nav-link">控制台</a>
          <a href="#claude" class="nav-link">Claude</a>
          <a href="#openai" class="nav-link">OpenAI</a>
          <a href="#grok" class="nav-link">Grok</a>
          <a href="#gemini" class="nav-link">Gemini</a>
          <a href="#features" class="nav-link">更多</a>
        </div>

        <div class="nav-right">
          <button class="theme-toggle" @click="toggleTheme" :title="isDark ? '切换到亮色' : '切换到暗色'">
            <svg v-if="isDark" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="5"/>
              <line x1="12" y1="1" x2="12" y2="3"/>
              <line x1="12" y1="21" x2="12" y2="23"/>
              <line x1="4.22" y1="4.22" x2="5.64" y2="5.64"/>
              <line x1="18.36" y1="18.36" x2="19.78" y2="19.78"/>
              <line x1="1" y1="12" x2="3" y2="12"/>
              <line x1="21" y1="12" x2="23" y2="12"/>
              <line x1="4.22" y1="19.78" x2="5.64" y2="18.36"/>
              <line x1="18.36" y1="5.64" x2="19.78" y2="4.22"/>
            </svg>
            <svg v-else viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/>
            </svg>
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn-login">
            {{ isAuthenticated ? '控制台' : '登录' }}
          </router-link>
        </div>
      </div>
    </nav>

    <!-- Side Navigation Dots -->
    <div class="side-nav">
      <div
        v-for="(section, index) in sections"
        :key="index"
        class="nav-dot"
        :class="{ active: currentSection === index }"
        @click="scrollToSection(index)"
      >
        <span class="nav-dot-label">{{ section }}</span>
      </div>
    </div>

    <!-- Screen 1: Hero with Opening Animation -->
    <section id="hero" class="hero-section" data-section="0">
      <ParticleNetwork
        ref="particleNetworkRef"
        :count="125"
        :max-particles="260"
        :explode-count="8"
        :connection-dist="110"
        :inner-radius="120"
        :outer-radius="170"
        :separation="50"
        :repel-strength="0.75"
        :snap-strength="0.02"
        :radius-rate="0.015"
        :resume-rate="0.012"
        :speed-multiplier="1"
        :bg-color="isDark ? '#0A0B0F' : '#FFFFFF'"
        :particle-color="isDark ? 'blue' : 'cream'"
        :show-mouse-line="false"
        :wrap-edges="true"
        class="particle-bg"
        @animation-complete="onParticleAnimationComplete"
      />

      <div class="hero-content" :class="{ visible: heroContentVisible }">
        <div class="hero-label">AI · API GATEWAY</div>
        <h1 class="hero-title">MODEL-GATE</h1>
        <p class="hero-subtitle">One Gateway, Every Model — Instantly Connected.</p>
        <div class="hero-actions">
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn-primary">
            <span>开始接入</span>
            <span class="arrow">→</span>
          </router-link>
          <a href="#dashboard" class="btn-secondary">
            <span>查看控制台</span>
            <span class="arrow">→</span>
          </a>
        </div>
      </div>
    </section>

    <!-- Screen 2: Dashboard Preview -->
    <section id="dashboard" class="dashboard-section" data-section="1">
      <div class="section-container">
        <div class="dashboard-header animate-section" :class="{ visible: visibleSections[1] }">
          <h2 class="dashboard-title">强大的控制台</h2>
          <p class="dashboard-subtitle">实时掌控每一次调用</p>
        </div>

        <div class="console-preview-frame" :style="previewSizing">
        <HomeConsolePreview
          :is-dark="isDark"
          site-name="MODEL-GATE"
          site-logo="/logo.svg"
          :active="currentSection === 1"
          @toggle-theme="toggleTheme"
        />
        </div>
      </div>
    </section>

    <!-- Screen 3: Claude Code -->
    <section id="claude" class="model-detail-section claude" data-section="2">
      <div class="model-detail-container">
        <div class="model-detail-left">
          <div class="model-detail-visual claude-visual animate-section" :class="{ visible: visibleSections[2] }">
            <svg class="claude-icon" viewBox="0 0 24 24" width="200" height="200">
              <path d="M4.709 15.955l4.72-2.647.08-.23-.08-.128H9.2l-.79-.048-2.698-.073-2.339-.097-2.266-.122-.571-.121L0 11.784l.055-.352.48-.321.686.06 1.52.103 2.278.158 1.652.097 2.449.255h.389l.055-.157-.134-.098-.103-.097-2.358-1.596-2.552-1.688-1.336-.972-.724-.491-.364-.462-.158-1.008.656-.722.881.06.225.061.893.686 1.908 1.476 2.491 1.833.365.304.145-.103.019-.073-.164-.274-1.355-2.446-1.446-2.49-.644-1.032-.17-.619a2.97 2.97 0 01-.104-.729L6.283.134 6.696 0l.996.134.42.364.62 1.414 1.002 2.229 1.555 3.03.456.898.243.832.091.255h.158V9.01l.128-1.706.237-2.095.23-2.695.08-.76.376-.91.747-.492.584.28.48.685-.067.444-.286 1.851-.559 2.903-.364 1.942h.212l.243-.242.985-1.306 1.652-2.064.73-.82.85-.904.547-.431h1.033l.76 1.129-.34 1.166-1.064 1.347-.881 1.142-1.264 1.7-.79 1.36.073.11.188-.02 2.856-.606 1.543-.28 1.841-.315.833.388.091.395-.328.807-1.969.486-2.309.462-3.439.813-.042.03.049.061 1.549.146.662.036h1.622l3.02.225.79.522.474.638-.079.485-1.215.62-1.64-.389-3.829-.91-1.312-.329h-.182v.11l1.093 1.068 2.006 1.81 2.509 2.33.127.578-.322.455-.34-.049-2.205-1.657-.851-.747-1.926-1.62h-.128v.17l.444.649 2.345 3.521.122 1.08-.17.353-.608.213-.668-.122-1.374-1.925-1.415-2.167-1.143-1.943-.14.08-.674 7.254-.316.37-.729.28-.607-.461-.322-.747.322-1.476.389-1.924.315-1.53.286-1.9.17-.632-.012-.042-.14.018-1.434 1.967-2.18 2.945-1.726 1.845-.414.164-.717-.37.067-.662.401-.589 2.388-3.036 1.44-1.882.93-1.086-.006-.158h-.055L4.132 18.56l-1.13.146-.487-.456.061-.746.231-.243 1.908-1.312-.006.006z" fill="#D97757" fill-rule="nonzero"/>
            </svg>
          </div>
        </div>
        <div class="model-detail-right">
          <div class="animate-section" :class="{ visible: visibleSections[2] }">
            <div class="model-tag ide">IDE 集成</div>
            <h2 class="model-detail-title">Claude Code</h2>
            <p class="model-detail-desc">通过 MODEL-GATE 接入 Claude Code，在终端中理解项目、编写代码与排查问题，让开发更专注。</p>

            <div class="code-blocks">
              <div class="code-block">
                <div class="code-header">
                  <span class="code-tab active">Mac / Linux</span>
                  <span class="code-tab">Windows</span>
                  <span class="code-tab">Node.js</span>
                </div>
                <div class="code-content">
                  <code>curl -fsSL https://claude.ai/install.sh | sh</code>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>

              <div class="code-block">
                <div class="code-header-simple">~/.claude/settings.json</div>
                <div class="code-content">
                  <pre>{
  "env": {
    "ANTHROPIC_AUTH_TOKEN": "sk-...",
    "ANTHROPIC_BASE_URL": "https://api.model-gate.cc"
  }
}</pre>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Screen 4: Codex CLI (OpenAI) -->
    <section id="openai" class="model-detail-section codex" data-section="3">
      <div class="model-detail-container">
        <div class="model-detail-left">
          <div class="model-detail-visual codex-visual animate-section" :class="{ visible: visibleSections[3] }">
            <svg class="codex-icon" viewBox="0 0 24 24" width="200" height="200" fill="currentColor">
              <path clip-rule="evenodd" d="M8.086.457a6.105 6.105 0 013.046-.415c1.333.153 2.521.72 3.564 1.7a.117.117 0 00.107.029c1.408-.346 2.762-.224 4.061.366l.063.03.154.076c1.357.703 2.33 1.77 2.918 3.198.278.679.418 1.388.421 2.126a5.655 5.655 0 01-.18 1.631.167.167 0 00.04.155 5.982 5.982 0 011.578 2.891c.385 1.901-.01 3.615-1.183 5.14l-.182.22a6.063 6.063 0 01-2.934 1.851.162.162 0 00-.108.102c-.255.736-.511 1.364-.987 1.992-1.199 1.582-2.962 2.462-4.948 2.451-1.583-.008-2.986-.587-4.21-1.736a.145.145 0 00-.14-.032c-.518.167-1.04.191-1.604.185a5.924 5.924 0 01-2.595-.622 6.058 6.058 0 01-2.146-1.781c-.203-.269-.404-.522-.551-.821a7.74 7.74 0 01-.495-1.283 6.11 6.11 0 01-.017-3.064.166.166 0 00.008-.074.115.115 0 00-.037-.064 5.958 5.958 0 01-1.38-2.202 5.196 5.196 0 01-.333-1.589 6.915 6.915 0 01.188-2.132c.45-1.484 1.309-2.648 2.577-3.493.282-.188.55-.334.802-.438.286-.12.573-.22.861-.304a.129.129 0 00.087-.087A6.016 6.016 0 015.635 2.31C6.315 1.464 7.132.846 8.086.457zm-.804 7.85a.848.848 0 00-1.473.842l1.694 2.965-1.688 2.848a.849.849 0 001.46.864l1.94-3.272a.849.849 0 00.007-.854l-1.94-3.393zm5.446 6.24a.849.849 0 000 1.695h4.848a.849.849 0 000-1.696h-4.848z" fill-rule="evenodd"/>
            </svg>
          </div>
        </div>
        <div class="model-detail-right">
          <div class="animate-section" :class="{ visible: visibleSections[3] }">
            <div class="model-tag cli">命令行工具</div>
            <h2 class="model-detail-title">Codex CLI</h2>
            <p class="model-detail-desc">Codex CLI 是一款在本地终端运行的编程助手，能够读取、修改并执行指定目录中的代码。</p>

            <div class="code-blocks">
              <div class="code-block dark">
                <div class="code-header">
                  <span class="code-tab active">Node.js</span>
                  <span class="code-tab">Mac</span>
                </div>
                <div class="code-content">
                  <code>npm install -g @openai/codex</code>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>

              <div class="code-block dark">
                <div class="code-header-simple">~/.codex/config.toml</div>
                <div class="code-content">
                  <pre>model_provider = "gateway"

[model_providers.gateway]
name = "OpenAI"
base_url = "https://api.model-gate.cc/v1"
wire_api = "responses"</pre>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>

              <div class="code-block dark">
                <div class="code-header-simple">~/.codex/auth.json</div>
                <div class="code-content">
                  <pre>{ "OPENAI_API_KEY": "sk-..." }</pre>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Screen 5: Grok -->
    <section id="grok" class="model-detail-section grok" data-section="4">
      <div class="model-detail-container">
        <div class="model-detail-left">
          <div class="model-detail-visual grok-visual animate-section" :class="{ visible: visibleSections[4] }">
            <svg class="grok-icon" viewBox="0 0 24 24" width="200" height="200" fill="currentColor">
              <path d="M9.27 15.29l7.978-5.897c.391-.29.95-.177 1.137.272.98 2.369.542 5.215-1.41 7.169-1.951 1.954-4.667 2.382-7.149 1.406l-2.711 1.257c3.889 2.661 8.611 2.003 11.562-.953 2.341-2.344 3.066-5.539 2.388-8.42l.006.007c-.983-4.232.242-5.924 2.75-9.383.06-.082.12-.164.179-.248l-3.301 3.305v-.01L9.267 15.292M7.623 16.723c-2.792-2.67-2.31-6.801.071-9.184 1.761-1.763 4.647-2.483 7.166-1.425l2.705-1.25a7.808 7.808 0 00-1.829-1A8.975 8.975 0 005.984 5.83c-2.533 2.536-3.33 6.436-1.962 9.764 1.022 2.487-.653 4.246-2.34 6.022-.599.63-1.199 1.259-1.682 1.925l7.62-6.815"/>
            </svg>
          </div>
        </div>
        <div class="model-detail-right">
          <div class="animate-section" :class="{ visible: visibleSections[4] }">
            <div class="model-tag openai">兼容 OpenAI</div>
            <h2 class="model-detail-title">Grok</h2>
            <p class="model-detail-desc">同一个终端点即可调用 Grok——只需更换模型名。</p>

            <div class="code-blocks">
              <div class="code-block dark">
                <div class="code-header-simple">curl</div>
                <div class="code-content">
                  <pre>curl https://api.model-gate.cc/v1/chat/completions \
  -H "Authorization: Bearer sk-..." \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-4",
    "messages": [{ "role": "user", "content": "Hello" }]
  }'</pre>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Screen 6: Gemini CLI -->
    <section id="gemini" class="model-detail-section gemini" data-section="5">
      <div class="model-detail-container">
        <div class="model-detail-left">
          <div class="model-detail-visual gemini-visual animate-section" :class="{ visible: visibleSections[5] }">
            <svg class="gemini-icon" viewBox="0 0 24 24" width="200" height="200">
              <path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="#3186FF"/>
              <path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="url(#gemini-gradient-0)"/>
              <path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="url(#gemini-gradient-1)"/>
              <path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="url(#gemini-gradient-2)"/>
              <defs>
                <linearGradient gradientUnits="userSpaceOnUse" id="gemini-gradient-0" x1="7" x2="11" y1="15.5" y2="12">
                  <stop stop-color="#08B962"/>
                  <stop offset="1" stop-color="#08B962" stop-opacity="0"/>
                </linearGradient>
                <linearGradient gradientUnits="userSpaceOnUse" id="gemini-gradient-1" x1="8" x2="11.5" y1="5.5" y2="11">
                  <stop stop-color="#F94543"/>
                  <stop offset="1" stop-color="#F94543" stop-opacity="0"/>
                </linearGradient>
                <linearGradient gradientUnits="userSpaceOnUse" id="gemini-gradient-2" x1="3.5" x2="17.5" y1="13.5" y2="12">
                  <stop stop-color="#FABC12"/>
                  <stop offset=".46" stop-color="#FABC12" stop-opacity="0"/>
                </linearGradient>
              </defs>
            </svg>
          </div>
        </div>
        <div class="model-detail-right">
          <div class="animate-section" :class="{ visible: visibleSections[5] }">
            <div class="model-tag multimodal">多模态 AI</div>
            <h2 class="model-detail-title">Gemini CLI</h2>
            <p class="model-detail-desc">在终端中使用 Gemini，快速接入多模态 AI。</p>

            <div class="code-blocks">
              <div class="code-block dark">
                <div class="code-header">
                  <span class="code-tab active">Node.js</span>
                  <span class="code-tab">Mac</span>
                </div>
                <div class="code-content">
                  <code>npm install -g @google/gemini-cli</code>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>

              <div class="code-block dark">
                <div class="code-header-simple">~/.gemini/.env</div>
                <div class="code-content">
                  <pre>GOOGLE_GEMINI_BASE_URL=https://api.model-gate.cc
GEMINI_API_KEY=sk-...</pre>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>

              <div class="code-block dark">
                <div class="code-header-simple">~/.gemini/settings.json</div>
                <div class="code-content">
                  <pre>{ "security": { "auth": {
  "selectedType": "gemini-api-key"
} } }</pre>
                  <button class="copy-btn" title="复制">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                      <rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
                      <path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
                    </svg>
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- Screen 7: Features + Models + Footer -->
    <section id="features" class="features-section" data-section="6">
      <div class="section-container">
        <!-- Feature Cards -->
        <div class="feature-cards animate-section" :class="{ visible: visibleSections[6] }">
          <div class="feature-card">
            <div class="feature-icon blue">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M4 7h16M4 12h16M4 17h16" stroke-linecap="round"/>
              </svg>
            </div>
            <h3>一键接入</h3>
            <p>获取一个 API 密钥，即可接入所有已接入的 AI 模型，无需分别申请。</p>
          </div>

          <div class="feature-card">
            <div class="feature-icon teal">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M12 2v20M2 12h20" stroke-linecap="round"/>
                <circle cx="12" cy="12" r="9"/>
              </svg>
            </div>
            <h3>稳定可靠</h3>
            <p>智能调度多个上游账号，自动故障切换与负载均衡，告别频繁报错。</p>
          </div>

          <div class="feature-card">
            <div class="feature-icon purple">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="3" width="7" height="7" rx="1"/>
                <rect x="14" y="3" width="7" height="7" rx="1"/>
                <rect x="14" y="14" width="7" height="7" rx="1"/>
                <rect x="3" y="14" width="7" height="7" rx="1"/>
              </svg>
            </div>
            <h3>用多少付多少</h3>
            <p>按实际 Token 计费，百分百配上账单，支持主流支付方式，余额可视化。</p>
          </div>
        </div>

        <!-- Models Marquee -->
        <div class="models-section animate-section" :class="{ visible: visibleSections[6] }" style="animation-delay: 0.2s">
          <div class="models-header">
            <h2 class="models-title">已支持的 AI 模型</h2>
            <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="model-plaza-link">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <rect x="3" y="3" width="7" height="7" rx="1"/>
                <rect x="14" y="3" width="7" height="7" rx="1"/>
                <rect x="14" y="14" width="7" height="7" rx="1"/>
                <rect x="3" y="14" width="7" height="7" rx="1"/>
              </svg>
              <span>模型广场</span>
            </router-link>
          </div>

          <div class="models-marquee">
            <div class="models-track">
              <div class="model-item" v-for="i in 2" :key="`set${i}`">
                <div class="model-icon-wrapper claude" v-html="modelIcons.claude"></div>
                <div class="model-icon-wrapper codex" v-html="modelIcons.codex"></div>
                <div class="model-icon-wrapper gemini" v-html="modelIcons.gemini"></div>
                <div class="model-icon-wrapper grok" v-html="modelIcons.grok"></div>
                <div class="model-icon-wrapper kimi" v-html="modelIcons.kimi"></div>
                <div class="model-icon-wrapper deepseek" v-html="modelIcons.deepseek"></div>
              </div>
            </div>
          </div>
        </div>

        <!-- Footer Content -->
        <div class="footer-content animate-section" :class="{ visible: visibleSections[6] }" style="animation-delay: 0.4s">
          <h2 class="footer-slogan">One Gateway, Every Model — Seamlessly.</h2>
          <div class="footer-links">
            <router-link to="/docs">文档</router-link>
            <span class="dot-separator">·</span>
            <a href="#">状态</a>
            <span class="dot-separator">·</span>
            <a href="#">联系</a>
          </div>
          <div class="footer-copyright">&copy; {{ currentYear }} MODEL-GATE</div>
        </div>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import { useWindowSize } from '@vueuse/core'
import { useAuthStore, useAppStore } from '@/stores'
import Icon from '@/components/icons/Icon.vue'
import ParticleNetwork from '@/components/ParticleNetwork.vue'
import HomeConsolePreview from '@/components/home/HomeConsolePreview.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'

const { t } = useI18n()
const { width: viewportWidth, height: viewportHeight } = useWindowSize()
const previewSizing = computed(() => {
  if (viewportWidth.value < 1024) return {}
  const availableHeight = Math.max(360, viewportHeight.value - (viewportHeight.value <= 800 ? 186 : 208))
  const availableWidth = Math.min(viewportWidth.value, 1920) - 96
  const scale = Math.max(0.5, Math.min(availableHeight / 904, availableWidth / 1500))
  return {
    '--preview-scale': String(scale),
    '--preview-panel-height': (availableHeight / scale - 104) + 'px'
  }
})

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')

const currentYear = computed(() => new Date().getFullYear())

// Section Navigation (7 sections now)
const sections = ['Hero', 'Dashboard', 'Claude', 'OpenAI', 'Grok', 'Gemini', 'Features']
const currentSection = ref(0)
const scrollContainer = ref<HTMLElement | null>(null)
const isScrolled = ref(false)
const heroContentVisible = ref(false)
const visibleSections = ref<{ [key: number]: boolean }>({})

// Model SVG Icons
const modelIcons = {
  claude: `<svg height="48" viewBox="0 0 24 24" width="48"><path d="M4.709 15.955l4.72-2.647.08-.23-.08-.128H9.2l-.79-.048-2.698-.073-2.339-.097-2.266-.122-.571-.121L0 11.784l.055-.352.48-.321.686.06 1.52.103 2.278.158 1.652.097 2.449.255h.389l.055-.157-.134-.098-.103-.097-2.358-1.596-2.552-1.688-1.336-.972-.724-.491-.364-.462-.158-1.008.656-.722.881.06.225.061.893.686 1.908 1.476 2.491 1.833.365.304.145-.103.019-.073-.164-.274-1.355-2.446-1.446-2.49-.644-1.032-.17-.619a2.97 2.97 0 01-.104-.729L6.283.134 6.696 0l.996.134.42.364.62 1.414 1.002 2.229 1.555 3.03.456.898.243.832.091.255h.158V9.01l.128-1.706.237-2.095.23-2.695.08-.76.376-.91.747-.492.584.28.48.685-.067.444-.286 1.851-.559 2.903-.364 1.942h.212l.243-.242.985-1.306 1.652-2.064.73-.82.85-.904.547-.431h1.033l.76 1.129-.34 1.166-1.064 1.347-.881 1.142-1.264 1.7-.79 1.36.073.11.188-.02 2.856-.606 1.543-.28 1.841-.315.833.388.091.395-.328.807-1.969.486-2.309.462-3.439.813-.042.03.049.061 1.549.146.662.036h1.622l3.02.225.79.522.474.638-.079.485-1.215.62-1.64-.389-3.829-.91-1.312-.329h-.182v.11l1.093 1.068 2.006 1.81 2.509 2.33.127.578-.322.455-.34-.049-2.205-1.657-.851-.747-1.926-1.62h-.128v.17l.444.649 2.345 3.521.122 1.08-.17.353-.608.213-.668-.122-1.374-1.925-1.415-2.167-1.143-1.943-.14.08-.674 7.254-.316.37-.729.28-.607-.461-.322-.747.322-1.476.389-1.924.315-1.53.286-1.9.17-.632-.012-.042-.14.018-1.434 1.967-2.18 2.945-1.726 1.845-.414.164-.717-.37.067-.662.401-.589 2.388-3.036 1.44-1.882.93-1.086-.006-.158h-.055L4.132 18.56l-1.13.146-.487-.456.061-.746.231-.243 1.908-1.312-.006.006z" fill="#D97757"/></svg><span>Claude</span>`,
  codex: `<svg height="48" viewBox="0 0 24 24" width="48"><path d="M19.503 0H4.496A4.496 4.496 0 000 4.496v15.007A4.496 4.496 0 004.496 24h15.007A4.496 4.496 0 0024 19.503V4.496A4.496 4.496 0 0019.503 0z" fill="#fff"/><path d="M9.064 3.344a4.578 4.578 0 012.285-.312c1 .115 1.891.54 2.673 1.275.01.01.024.017.037.021a.09.09 0 00.043 0 4.55 4.55 0 013.046.275l.047.022.116.057a4.581 4.581 0 012.188 2.399c.209.51.313 1.041.315 1.595a4.24 4.24 0 01-.134 1.223.123.123 0 00.03.115c.594.607.988 1.33 1.183 2.17.289 1.425-.007 2.71-.887 3.854l-.136.166a4.548 4.548 0 01-2.201 1.388.123.123 0 00-.081.076c-.191.551-.383 1.023-.74 1.494-.9 1.187-2.222 1.846-3.711 1.838-1.187-.006-2.239-.44-3.157-1.302a.107.107 0 00-.105-.024c-.388.125-.78.143-1.204.138a4.441 4.441 0 01-1.945-.466 4.544 4.544 0 01-1.61-1.335c-.152-.202-.303-.392-.414-.617a5.81 5.81 0 01-.37-.961 4.582 4.582 0 01-.014-2.298.124.124 0 00.006-.056.085.085 0 00-.027-.048 4.467 4.467 0 01-1.034-1.651 3.896 3.896 0 01-.251-1.192 5.189 5.189 0 01.141-1.6c.337-1.112.982-1.985 1.933-2.618.212-.141.413-.251.601-.33.215-.089.43-.164.646-.227a.098.098 0 00.065-.066 4.51 4.51 0 01.829-1.615 4.535 4.535 0 011.837-1.388zm3.482 10.565a.637.637 0 000 1.272h3.636a.637.637 0 100-1.272h-3.636zM8.462 9.23a.637.637 0 00-1.106.631l1.272 2.224-1.266 2.136a.636.636 0 101.095.649l1.454-2.455a.636.636 0 00.005-.64L8.462 9.23z" fill="url(#codex-g)"/><defs><linearGradient id="codex-g" x1="12" x2="12" y1="3" y2="21"><stop stop-color="#B1A7FF"/><stop offset=".5" stop-color="#7A9DFF"/><stop offset="1" stop-color="#3941FF"/></linearGradient></defs></svg><span>Codex</span>`,
  gemini: `<svg height="48" viewBox="0 0 24 24" width="48"><path d="M20.616 10.835a14.147 14.147 0 01-4.45-3.001 14.111 14.111 0 01-3.678-6.452.503.503 0 00-.975 0 14.134 14.134 0 01-3.679 6.452 14.155 14.155 0 01-4.45 3.001c-.65.28-1.318.505-2.002.678a.502.502 0 000 .975c.684.172 1.35.397 2.002.677a14.147 14.147 0 014.45 3.001 14.112 14.112 0 013.679 6.453.502.502 0 00.975 0c.172-.685.397-1.351.677-2.003a14.145 14.145 0 013.001-4.45 14.113 14.113 0 016.453-3.678.503.503 0 000-.975 13.245 13.245 0 01-2.003-.678z" fill="#3186FF"/></svg><span>Gemini</span>`,
  grok: `<svg height="48" viewBox="0 0 24 24" width="48" fill="currentColor"><path d="M9.27 15.29l7.978-5.897c.391-.29.95-.177 1.137.272.98 2.369.542 5.215-1.41 7.169-1.951 1.954-4.667 2.382-7.149 1.406l-2.711 1.257c3.889 2.661 8.611 2.003 11.562-.953 2.341-2.344 3.066-5.539 2.388-8.42l.006.007c-.983-4.232.242-5.924 2.75-9.383.06-.082.12-.164.179-.248l-3.301 3.305v-.01L9.267 15.292M7.623 16.723c-2.792-2.67-2.31-6.801.071-9.184 1.761-1.763 4.647-2.483 7.166-1.425l2.705-1.25a7.808 7.808 0 00-1.829-1A8.975 8.975 0 005.984 5.83c-2.533 2.536-3.33 6.436-1.962 9.764 1.022 2.487-.653 4.246-2.34 6.022-.599.63-1.199 1.259-1.682 1.925l7.62-6.815"/></svg><span>Grok</span>`,
  kimi: `<svg height="48" viewBox="0 0 24 24" width="48"><path d="M21.846 0a1.923 1.923 0 110 3.846H20.15a.226.226 0 01-.227-.226V1.923C19.923.861 20.784 0 21.846 0z" fill="#1783FF"/><path d="M11.065 11.199l7.257-7.2c.137-.136.06-.41-.116-.41H14.3a.164.164 0 00-.117.051l-7.82 7.756c-.122.12-.302.013-.302-.179V3.82c0-.127-.083-.23-.185-.23H3.186c-.103 0-.186.103-.186.23V19.77c0 .128.083.23.186.23h2.69c.103 0 .186-.102.186-.23v-3.25c0-.069.025-.135.069-.178l2.424-2.406a.158.158 0 01.205-.023l6.484 4.772a7.677 7.677 0 003.453 1.283c.108.012.2-.095.2-.23v-3.06c0-.117-.07-.212-.164-.227a5.028 5.028 0 01-2.027-.807l-5.613-4.064c-.117-.078-.132-.279-.028-.381z" fill="#fff"/></svg><span>Kimi</span>`,
  deepseek: `<svg height="48" viewBox="0 0 24 24" width="48"><path d="M23.748 4.482c-.254-.124-.364.113-.512.234-.051.039-.094.09-.137.136-.372.397-.806.657-1.373.626-.829-.046-1.537.214-2.163.848-.133-.782-.575-1.248-1.247-1.548-.352-.156-.708-.311-.955-.65-.172-.241-.219-.51-.305-.774-.055-.16-.11-.323-.293-.35-.2-.031-.278.136-.356.276-.313.572-.434 1.202-.422 1.84.027 1.436.633 2.58 1.838 3.393.137.093.172.187.129.323-.082.28-.18.552-.266.833-.055.179-.137.217-.329.14a5.526 5.526 0 01-1.736-1.18c-.857-.828-1.631-1.742-2.597-2.458a11.365 11.365 0 00-.689-.471c-.985-.957.13-1.743.388-1.836.27-.098.093-.432-.779-.428-.872.004-1.67.295-2.687.684a3.055 3.055 0 01-.465.137 9.597 9.597 0 00-2.883-.102c-1.885.21-3.39 1.102-4.497 2.623C.082 8.606-.231 10.684.152 12.85c.403 2.284 1.569 4.175 3.36 5.653 1.858 1.533 3.997 2.284 6.438 2.14 1.482-.085 3.133-.284 4.994-1.86.47.234.962.327 1.78.397.63.059 1.236-.03 1.705-.128.735-.156.684-.837.419-.961-2.155-1.004-1.682-.595-2.113-.926 1.096-1.296 2.746-2.642 3.392-7.003.05-.347.007-.565 0-.845-.004-.17.035-.237.23-.256a4.173 4.173 0 001.545-.475c1.396-.763 1.96-2.015 2.093-3.517.02-.23-.004-.467-.247-.588zM11.581 18c-2.089-1.642-3.102-2.183-3.52-2.16-.392.024-.321.471-.235.763.09.288.207.486.371.739.114.167.192.416-.113.603-.673.416-1.842-.14-1.897-.167-1.361-.802-2.5-1.86-3.301-3.307-.774-1.393-1.224-2.887-1.298-4.482-.02-.386.093-.522.477-.592a4.696 4.696 0 011.529-.039c2.132.312 3.946 1.265 5.468 2.774.868.86 1.525 1.887 2.202 2.891.72 1.066 1.494 2.082 2.48 2.914.348.292.625.514.891.677-.802.09-2.14.11-3.054-.614zm1-6.44a.306.306 0 01.415-.287.302.302 0 01.2.288.306.306 0 01-.31.307.303.303 0 01-.304-.308zm3.11 1.596c-.2.081-.399.151-.59.16a1.245 1.245 0 01-.798-.254c-.274-.23-.47-.358-.552-.758a1.73 1.73 0 01.016-.588c.07-.327-.008-.537-.239-.727-.187-.156-.426-.199-.688-.199a.559.559 0 01-.254-.078c-.11-.054-.2-.19-.114-.358.028-.054.16-.186.192-.21.356-.202.767-.136 1.146.016.352.144.618.408 1.001.782.391.451.462.576.685.914.176.265.336.537.445.848.067.195-.019.354-.25.452z" fill="#4D6BFE"/></svg><span>DeepSeek</span>`,
}

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

// Initialize theme
function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  isDark.value = savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  document.documentElement.classList.toggle('dark', isDark.value)
}

// Scroll to section
function scrollToSection(index: number) {
  const container = scrollContainer.value
  if (!container) return

  const sections = container.querySelectorAll(':scope > section[data-section]')
  if (sections[index]) {
    sections[index].scrollIntoView({ behavior: 'smooth' })
  }
}

// Update current section and nav state on scroll
function handleScroll() {
  const container = scrollContainer.value
  if (!container) return

  // Update nav scrolled state
  isScrolled.value = container.scrollTop > 50

  const sections = container.querySelectorAll(':scope > section[data-section]')
  const scrollPosition = container.scrollTop + window.innerHeight / 2

  sections.forEach((section, index) => {
    const sectionTop = (section as HTMLElement).offsetTop
    const sectionBottom = sectionTop + (section as HTMLElement).offsetHeight

    if (scrollPosition >= sectionTop && scrollPosition < sectionBottom) {
      currentSection.value = index

      // Trigger section visibility animation
      if (!visibleSections.value[index]) {
        visibleSections.value[index] = true
      }
    }
  })
}

// Particle animation complete
function onParticleAnimationComplete() {
  heroContentVisible.value = true
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()

  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  nextTick(() => {
    const container = scrollContainer.value
    if (container) {
      container.addEventListener('scroll', handleScroll)
    }

    // Initial visibility check
    handleScroll()
  })
})

onBeforeUnmount(() => {
  const container = scrollContainer.value
  if (container) {
    container.removeEventListener('scroll', handleScroll)
  }
})
</script>

<style scoped>
/* ===== Design Tokens ===== */
.model-gate-home.is-dark {
  --bg: #0a0b0f;
  --bg-alt: #0f1419;
  --surface: #12141b;
  --text: #e8ecf4;
  --text-dim: #8a93a6;
  --accent: #4da3ff;
  --accent-soft: rgba(77, 163, 255, 0.16);
  --line: rgba(80, 140, 235, 0.28);
  --glow: 0 0 24px rgba(77, 163, 255, 0.45);
}

/* ===== Global Container ===== */
.model-gate-home {
  --bg: #ffffff;
  --bg-alt: #f7f8fa;
  --surface: #ffffff;
  --text: #172033;
  --text-dim: #526176;
  --accent: #2563b4;
  --accent-soft: rgba(37, 99, 180, 0.1);
  --line: rgba(71, 85, 105, 0.2);
  --glow: 0 0 24px rgba(37, 99, 180, 0.14);
  scroll-snap-type: y mandatory;
  overflow-y: scroll;
  height: 100vh;
  background: var(--bg-alt);
  color: var(--text);
  font-family: 'Inter', 'Space Grotesk', 'PingFang SC', 'Microsoft YaHei', sans-serif;
  position: relative;
}

.model-gate-home section {
  scroll-snap-align: start;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.section-container {
  width: 100%;
  max-width: 1200px;
  padding: clamp(24px, 6vw, 96px);
  margin: 0 auto;
}

/* ===== Animations ===== */
.animate-section {
  opacity: 0;
  transform: translateY(30px);
  transition: all 0.8s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.animate-section.visible {
  opacity: 1;
  transform: translateY(0);
}

/* ===== Top Navigation Bar ===== */
.top-nav {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  z-index: 1000;
  background: transparent;
  transition: all 0.3s ease;
}

.top-nav.scrolled {
  background: rgba(10, 11, 15, 0.9);
  backdrop-filter: blur(12px);
  border-bottom: 1px solid rgba(80, 140, 235, 0.12);
}

.nav-container {
  max-width: 1400px;
  margin: 0 auto;
  padding: 0 32px;
  height: 64px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 48px;
}

.nav-left {
  flex-shrink: 0;
}

.nav-logo {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}

.logo-image {
  width: 36px;
  height: 36px;
  object-fit: contain;
  flex-shrink: 0;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-weight: 700;
  font-size: 14px;
  color: white;
}

.logo-text {
  font-size: 18px;
  font-weight: 600;
  color: var(--text);
}

.nav-center {
  display: flex;
  align-items: center;
  gap: 32px;
  flex: 1;
  justify-content: center;
}

.nav-link {
  font-size: 14px;
  color: var(--text-dim);
  text-decoration: none;
  transition: color 0.2s;
  white-space: nowrap;
}

.nav-link:hover {
  color: var(--text);
}

.nav-link.active {
  color: var(--accent);
}

.nav-right {
  display: flex;
  align-items: center;
  gap: 16px;
  flex-shrink: 0;
}

.theme-toggle,
.lang-toggle {
  width: 36px;
  height: 36px;
  border-radius: 8px;
  background: transparent;
  border: 1px solid rgba(255, 255, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text-dim);
}

.theme-toggle svg,
.lang-toggle svg {
  width: 18px;
  height: 18px;
}

.theme-toggle:hover,
.lang-toggle:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: rgba(255, 255, 255, 0.24);
  color: var(--text);
}

.btn-login {
  padding: 10px 20px;
  background: linear-gradient(135deg, #14B8A6, #0D9488);
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  color: white;
  text-decoration: none;
  transition: all 0.2s;
  white-space: nowrap;
}

.btn-login:hover {
  transform: translateY(-2px);
  box-shadow: 0 4px 12px rgba(20, 184, 166, 0.4);
}

@media (max-width: 968px) {
  .nav-center {
    display: none;
  }
}

/* ===== Side Navigation Dots ===== */
.side-nav {
  position: fixed;
  right: 24px;
  top: 50%;
  transform: translateY(-50%);
  z-index: 100;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.nav-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: rgba(255, 255, 255, 0.2);
  cursor: pointer;
  transition: all 0.3s ease;
  position: relative;
}

.nav-dot::before {
  content: '';
  position: absolute;
  inset: -4px;
  border-radius: 50%;
  border: 2px solid transparent;
  transition: all 0.3s ease;
}

.nav-dot:hover {
  background: rgba(255, 255, 255, 0.4);
}

.nav-dot.active {
  background: var(--accent);
  width: 12px;
  height: 12px;
}

.nav-dot.active::before {
  border-color: var(--accent);
}

/* Hover label */
.nav-dot-label {
  position: absolute;
  right: 24px;
  top: 50%;
  transform: translateY(-50%);
  background: rgba(0, 0, 0, 0.9);
  color: white;
  padding: 6px 12px;
  border-radius: 6px;
  font-size: 13px;
  white-space: nowrap;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s;
}

.nav-dot:hover .nav-dot-label {
  opacity: 1;
}

/* Light mode adaptation */
.model-gate-home:not(.is-dark) .nav-dot {
  background: rgba(0, 0, 0, 0.2);
}

.model-gate-home:not(.is-dark) .nav-dot:hover {
  background: rgba(0, 0, 0, 0.4);
}

/* ===== Hero Section ===== */
.hero-section {
  position: relative;
  overflow: hidden;
  background: var(--bg);
}

.model-gate-home:not(.is-dark) .hero-section {
  background: #FFFFFF;
}

.particle-bg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  z-index: 1;
  pointer-events: auto;
}

.hero-content {
  position: relative;
  z-index: 10;
  text-align: center;
  padding: clamp(24px, 6vw, 96px);
  max-width: 900px;
  pointer-events: none;
  opacity: 0;
  transform: translateY(30px);
  transition: all 1s ease-out;
}

.hero-content.visible {
  opacity: 1;
  transform: translateY(0);
}

.hero-content > * {
  pointer-events: auto;
}

.hero-label {
  font-size: 13px;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--text-dim);
  margin-bottom: 24px;
  font-weight: 600;
}

.model-gate-home:not(.is-dark) .hero-label {
  color: #6B7280;
}

.hero-title {
  font-size: clamp(56px, 10vw, 96px);
  font-weight: 700;
  line-height: 1;
  margin-bottom: 24px;
  color: var(--text);
  letter-spacing: -0.02em;
  white-space: nowrap;
}

.model-gate-home:not(.is-dark) .hero-title {
  color: #111827;
}

.hero-subtitle {
  font-size: clamp(16px, 2.5vw, 20px);
  color: var(--text-dim);
  margin-bottom: 40px;
  line-height: 1.5;
}

.model-gate-home:not(.is-dark) .hero-subtitle {
  color: #4B5563;
}

.hero-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  flex-wrap: wrap;
}

.btn-primary,
.btn-secondary {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 14px 28px;
  border-radius: 999px;
  font-size: 15px;
  font-weight: 500;
  text-decoration: none;
  transition: all 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.btn-primary {
  background: transparent;
  border: 1.5px solid var(--accent);
  color: var(--accent);
  box-shadow: var(--glow);
}

.btn-primary:hover {
  background: var(--accent-soft);
  box-shadow: 0 0 32px rgba(77, 163, 255, 0.6);
}

.btn-secondary {
  background: transparent;
  border: 1.5px solid rgba(255, 255, 255, 0.12);
  color: var(--text);
}

.btn-secondary:hover {
  border-color: rgba(255, 255, 255, 0.24);
  background: rgba(255, 255, 255, 0.04);
}

.arrow {
  transition: transform 0.25s cubic-bezier(0.25, 0.46, 0.45, 0.94);
}

.btn-primary:hover .arrow,
.btn-secondary:hover .arrow {
  transform: translateX(6px);
}

/* ===== Dashboard Section ===== */
.model-gate-home .dashboard-section {
  background: var(--bg-alt);
  min-height: 100vh;
  align-items: flex-start;
}

.dashboard-header {
  text-align: center;
  margin-bottom: 10px;
}

.dashboard-title {
  font-size: clamp(20px, 2vw, 24px);
  font-weight: 600;
  margin-bottom: 2px;
  color: var(--text);
}

.dashboard-subtitle {
  font-size: 13px;
  color: var(--text-dim);
}

.dashboard-section > .section-container {
  max-width: 1920px;
  min-width: 0;
  padding: 72px clamp(12px, 1.5vw, 28px) 16px;
}

.model-gate-home:not(.is-dark) .top-nav.scrolled {
  background: rgba(255, 255, 255, 0.92);
  border-bottom-color: var(--line);
}

.model-gate-home:not(.is-dark) .btn-login {
  color: #fff;
  background: #172033;
}

.model-gate-home:not(.is-dark) .btn-primary {
  color: var(--accent);
}

.model-gate-home:not(.is-dark) .code-block,
.model-gate-home:not(.is-dark) .code-block.dark {
  background: var(--surface);
  border-color: var(--line);
}

.model-gate-home:not(.is-dark) .code-header,
.model-gate-home:not(.is-dark) .code-header-simple {
  background: #eef1f6;
  border-bottom-color: var(--line);
}

.model-gate-home:not(.is-dark) .model-tag.ide { color: #9b482e; }
.model-gate-home:not(.is-dark) .model-tag.openai { color: #9d5500; }
.model-gate-home:not(.is-dark) .model-tag.multimodal { color: #7c3fb2; }
.model-gate-home:not(.is-dark) .model-icon-wrapper.grok svg { color: var(--text); }

.model-gate-home:not(.is-dark) .btn-secondary,
.model-gate-home:not(.is-dark) .theme-toggle,
.model-gate-home:not(.is-dark) .lang-toggle {
  border-color: var(--line);
}

.model-gate-home:not(.is-dark) .nav-dot.active {
  background: var(--accent);
}

@media (max-width: 600px) {
  .nav-container { padding: 0 16px; gap: 12px; }
  .nav-logo { gap: 8px; }
  .nav-right { gap: 8px; }
  .btn-login { padding: 9px 14px; }
  .lang-toggle { display: none; }
  .hero-content { padding: 24px 16px; width: 100%; }
  .hero-title { font-size: clamp(32px, 10vw, 56px); }
  .side-nav { right: 6px; }
}

/* ===== Model Detail Sections ===== */
.model-detail-section {
  background: var(--bg-alt);
}

.model-detail-container {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 80px;
  align-items: center;
  max-width: 1200px;
  margin: 0 auto;
  padding: clamp(24px, 6vw, 96px);
}

@media (max-width: 968px) {
  .model-detail-container {
    grid-template-columns: 1fr;
    gap: 48px;
  }
}

.model-detail-left {
  display: flex;
  justify-content: center;
  align-items: center;
}

.model-detail-visual {
  width: 100%;
  max-width: 400px;
  aspect-ratio: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

/* SVG Animations */
.claude-icon,
.codex-icon,
.grok-icon,
.gemini-icon {
  width: 100%;
  height: 100%;
  animation: float-pulse 6s ease-in-out infinite;
}

@keyframes float-pulse {
  0%, 100% {
    transform: translateY(0) scale(1);
    opacity: 0.9;
  }
  50% {
    transform: translateY(-20px) scale(1.05);
    opacity: 1;
  }
}

.codex-icon {
  color: #E8E8E8;
}

.model-detail-right {
  flex: 1;
}

.model-tag {
  display: inline-block;
  padding: 6px 16px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: 20px;
}

.model-tag.ide {
  background: rgba(224, 120, 86, 0.16);
  color: #E07856;
  border: 1px solid rgba(224, 120, 86, 0.3);
}

.model-tag.cli {
  background: rgba(255, 255, 255, 0.08);
  color: var(--text);
  border: 1px solid rgba(255, 255, 255, 0.16);
}

.model-tag.openai {
  background: rgba(255, 165, 75, 0.16);
  color: #FFA54B;
  border: 1px solid rgba(255, 165, 75, 0.3);
}

.model-tag.multimodal {
  background: linear-gradient(135deg, rgba(255, 107, 107, 0.16), rgba(157, 78, 221, 0.16));
  color: #A855F7;
  border: 1px solid rgba(157, 78, 221, 0.3);
}

.model-detail-title {
  font-size: clamp(36px, 6vw, 56px);
  font-weight: 600;
  margin-bottom: 20px;
  color: var(--text);
}

.model-detail-desc {
  font-size: 16px;
  line-height: 1.7;
  color: var(--text-dim);
  margin-bottom: 40px;
}

.code-blocks {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.code-block {
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(80, 140, 235, 0.16);
  border-radius: 12px;
  overflow: hidden;
}

.code-block.dark {
  background: #12141b;
  border-color: rgba(255, 255, 255, 0.08);
}

.code-header {
  display: flex;
  gap: 8px;
  padding: 12px 16px;
  background: rgba(0, 0, 0, 0.2);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.code-tab {
  font-size: 13px;
  padding: 4px 12px;
  border-radius: 6px;
  color: var(--text-dim);
  cursor: pointer;
  transition: all 0.2s;
}

.code-tab.active {
  background: var(--accent-soft);
  color: var(--accent);
}

.code-header-simple {
  font-family: 'JetBrains Mono', 'SF Mono', monospace;
  font-size: 12px;
  padding: 12px 16px;
  color: var(--text-dim);
  background: rgba(0, 0, 0, 0.2);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);
}

.code-content {
  position: relative;
  padding: 20px;
  font-family: 'JetBrains Mono', 'SF Mono', monospace;
  font-size: 14px;
  line-height: 1.6;
  color: var(--text);
  overflow-x: auto;
}

.code-content code {
  display: block;
}

.code-content pre {
  margin: 0;
  white-space: pre-wrap;
  word-wrap: break-word;
}

.copy-btn {
  position: absolute;
  top: 16px;
  right: 16px;
  padding: 8px;
  background: rgba(255, 255, 255, 0.08);
  border: 1px solid rgba(255, 255, 255, 0.12);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s;
  color: var(--text-dim);
}

.copy-btn:hover {
  background: rgba(255, 255, 255, 0.12);
  color: var(--text);
}

.copy-btn svg {
  width: 16px;
  height: 16px;
}

/* ===== Features Section ===== */
.features-section {
  background: var(--bg-alt);
}

.feature-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 24px;
  margin-bottom: 80px;
}

.feature-card {
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(80, 140, 235, 0.16);
  border-radius: 16px;
  padding: 32px;
  transition: all 0.3s ease;
}

.feature-card:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: var(--accent-soft);
  transform: translateY(-4px);
}

.feature-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 20px;
}

.feature-icon svg {
  width: 28px;
  height: 28px;
  color: white;
}

.feature-icon.blue {
  background: linear-gradient(135deg, #3B82F6, #2563EB);
}

.feature-icon.teal {
  background: linear-gradient(135deg, #14B8A6, #0D9488);
}

.feature-icon.purple {
  background: linear-gradient(135deg, #A855F7, #9333EA);
}

.feature-card h3 {
  font-size: 20px;
  font-weight: 600;
  margin-bottom: 12px;
  color: var(--text);
}

.feature-card p {
  font-size: 14px;
  line-height: 1.7;
  color: var(--text-dim);
}

/* ===== Models Marquee ===== */
.models-section {
  margin-bottom: 80px;
}

.models-header {
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 24px;
  margin-bottom: 48px;
  flex-wrap: wrap;
}

.models-title {
  font-size: clamp(32px, 5vw, 40px);
  font-weight: 600;
  color: var(--text);
}

.model-plaza-link {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 20px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(80, 140, 235, 0.16);
  border-radius: 8px;
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
  text-decoration: none;
  transition: all 0.3s ease;
}

.model-plaza-link svg {
  width: 16px;
  height: 16px;
}

.model-plaza-link:hover {
  background: rgba(255, 255, 255, 0.08);
  border-color: var(--accent-soft);
  transform: translateY(-2px);
}

.models-marquee {
  overflow: hidden;
  position: relative;
  padding: 24px 0;
}

.models-track {
  display: flex;
  gap: 24px;
  animation: scroll-models 26s linear infinite;
  width: fit-content;
}

.model-item {
  display: flex;
  gap: 24px;
  flex-shrink: 0;
}

@keyframes scroll-models {
  0% { transform: translateX(calc(-50% - 12px)); }
  100% { transform: translateX(0); }
}

.model-icon-wrapper {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 12px;
  background: rgba(255, 255, 255, 0.02);
  border: 1px solid rgba(80, 140, 235, 0.12);
  border-radius: 16px;
  width: 104px;
  min-width: 104px;
  height: 104px;
  transition: all 0.3s ease;
}

.model-icon-wrapper:hover {
  background: rgba(255, 255, 255, 0.04);
  border-color: var(--accent-soft);
  transform: translateY(-4px);
}

.model-icon-wrapper span {
  font-size: 14px;
  font-weight: 500;
  color: var(--text);
}

.model-icon-wrapper.claude {
  border-color: rgba(217, 119, 87, 0.3);
}

.model-icon-wrapper.codex {
  border-color: rgba(122, 157, 255, 0.3);
}

.model-icon-wrapper.gemini {
  border-color: rgba(49, 134, 255, 0.3);
}

.model-icon-wrapper.grok {
  border-color: rgba(255, 255, 255, 0.2);
}

.model-icon-wrapper.grok svg {
  color: #E8E8E8;
}

.model-icon-wrapper.kimi {
  border-color: rgba(23, 131, 255, 0.3);
}

.model-icon-wrapper.deepseek {
  border-color: rgba(77, 107, 254, 0.3);
}

/* ===== Footer ===== */
.footer-content {
  text-align: center;
  padding: 64px 0;
  border-top: 1px solid rgba(80, 140, 235, 0.16);
}

.footer-slogan {
  font-size: clamp(24px, 4vw, 36px);
  font-weight: 600;
  color: var(--text);
  margin-bottom: 32px;
  line-height: 1.3;
}

.footer-links {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 16px;
  flex-wrap: wrap;
  margin-bottom: 24px;
  font-family: 'JetBrains Mono', 'SF Mono', monospace;
  font-size: 14px;
}

.footer-links a {
  color: var(--text-dim);
  text-decoration: none;
  transition: color 0.2s;
}

.footer-links a:hover {
  color: var(--text);
}

.dot-separator {
  color: var(--text-dim);
  opacity: 0.5;
}

.footer-copyright {
  font-size: 13px;
  color: var(--text-dim);
  opacity: 0.7;
}
.console-preview-frame { width: 100%; min-width: 0; }
@media (min-width: 1024px) {
  .console-preview-frame :deep(.console-preview) { zoom: var(--preview-scale, 0.72); width: 100%; }
}
@media (min-width: 768px) and (max-width: 1023px) {
  .console-preview-frame :deep(.console-preview) { zoom: 0.85; width: 100%; }
}
@media (min-width: 1024px) and (max-height: 800px) {
  .dashboard-header { margin-bottom: 8px; }
  .dashboard-title { font-size: 20px; margin-bottom: 2px; }
  .dashboard-section > .section-container { padding-top: 70px; }
}
.model-gate-home:not(.is-dark) .model-icon-wrapper.grok {
  border-color: #8493ac;
  background: #e9edf4;
}
.models-marquee::after {
  content: '';
  position: absolute;
  inset: 0 0 0 50%;
  pointer-events: none;
  z-index: 2;
  backdrop-filter: blur(5px);
  background-image: repeating-linear-gradient(0deg, transparent 0 9px, var(--bg-alt) 9px 11px), repeating-linear-gradient(90deg, transparent 0 9px, var(--bg-alt) 9px 11px);
  mask-image: linear-gradient(to right, transparent, #000 30%);
}
.model-icon-wrapper :deep(svg) { width: 42px; height: 42px; flex-shrink: 0; }
.model-icon-wrapper :deep(span) { font-size: 13px; color: var(--text); }
.model-gate-home:not(.is-dark) .model-icon-wrapper.grok :deep(svg) { color: #243047; }
.gemini .model-detail-container { padding-top: 76px; padding-bottom: 24px; gap: 48px; }
.gemini .model-tag { margin-bottom: 10px; }
.gemini .model-detail-title { font-size: clamp(28px, 4vw, 42px); margin-bottom: 10px; }
.gemini .model-detail-desc { font-size: 14px; margin-bottom: 16px; }
.gemini .code-blocks { gap: 10px; }
.gemini .code-content { padding: 10px 16px; font-size: 12px; line-height: 1.45; }
.gemini .code-header, .gemini .code-header-simple { padding: 7px 12px; }
@media (max-width: 968px) {
  .gemini .model-detail-container { gap: 16px; padding: 76px 20px 20px; }
  .gemini .model-detail-visual svg { width: 72px; height: 72px; }
}
@media (min-width: 1024px) {
  .console-preview-frame { width: calc(100% - 32px); margin-inline: auto; }
}
</style>
