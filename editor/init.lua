-- goTutor Neovim config: plugin-free, needs Neovim 0.11+ and gopls on PATH.
-- Loaded with `nvim -u editor/init.lua`, so it never touches your own config.
--
--   <Space>t   go test (race, verbose) for this lesson -> quickfix
--   <Space>r   go run .
--   <Space>v   go vet
--   <Space>n   jump to the next TODO(you)
--   gd / K / gr / <Space>a   definition / docs / references / code action (gopls)
--   ]d [d      next / previous diagnostic

vim.g.mapleader = ' '
local o = vim.opt
o.number = true
o.relativenumber = true
o.signcolumn = 'yes'
o.termguicolors = true
o.mouse = 'a'
o.undofile = false
o.swapfile = false
o.autoread = true
o.updatetime = 300
o.splitright = true
o.scrolloff = 5
o.completeopt = { 'menuone', 'noselect', 'popup' }
o.statusline = ' goTutor │ %f %m%=%{v:lua.GotutorDiag()} │ %l:%c '

vim.cmd.colorscheme('habamax')
vim.cmd([[syntax match TutorTodo /TODO(you)/ containedin=ALL]])
vim.api.nvim_set_hl(0, 'TutorTodo', { fg = '#000000', bg = '#ffaf00', bold = true })

-- Reload files the tutor changed on disk.
vim.api.nvim_create_autocmd({ 'FocusGained', 'BufEnter', 'CursorHold' }, {
  command = 'silent! checktime',
})

-- Go buffers: real tabs, and the TODO(you) highlight.
vim.api.nvim_create_autocmd('FileType', {
  pattern = 'go',
  callback = function()
    vim.bo.expandtab = false
    vim.bo.tabstop = 4
    vim.bo.shiftwidth = 4
    vim.fn.matchadd('TutorTodo', 'TODO(you)')
  end,
})

-- gopls, via the built-in client.
vim.lsp.config('gopls', {
  cmd = { 'gopls' },
  filetypes = { 'go', 'gomod' },
  root_markers = { 'go.mod' },
  settings = {
    gopls = {
      staticcheck = true,
      gofumpt = false,
      analyses = { unusedparams = true, shadow = true },
      hints = { parameterNames = true, assignVariableTypes = true },
    },
  },
})
vim.lsp.enable('gopls')

vim.api.nvim_create_autocmd('LspAttach', {
  callback = function(ev)
    local client = vim.lsp.get_client_by_id(ev.data.client_id)
    local map = function(lhs, fn) vim.keymap.set('n', lhs, fn, { buffer = ev.buf }) end
    map('gd', vim.lsp.buf.definition)
    map('gr', vim.lsp.buf.references)
    map('K', vim.lsp.buf.hover)
    map('<leader>a', vim.lsp.buf.code_action)
    map('<leader>R', vim.lsp.buf.rename)
    if client and client:supports_method('textDocument/completion') then
      vim.lsp.completion.enable(true, client.id, ev.buf, { autotrigger = true })
    end
  end,
})

vim.diagnostic.config({ virtual_text = true, severity_sort = true })

-- Format and organize imports on save.
vim.api.nvim_create_autocmd('BufWritePre', {
  pattern = '*.go',
  callback = function()
    local params = vim.lsp.util.make_range_params(0, 'utf-8')
    params.context = { only = { 'source.organizeImports' } }
    local res = vim.lsp.buf_request_sync(0, 'textDocument/codeAction', params, 1000)
    for cid, r in pairs(res or {}) do
      for _, action in pairs(r.result or {}) do
        if action.edit then
          local enc = (vim.lsp.get_client_by_id(cid) or {}).offset_encoding or 'utf-16'
          vim.lsp.util.apply_workspace_edit(action.edit, enc)
        end
      end
    end
    vim.lsp.buf.format({ async = false, timeout_ms = 1000 })
  end,
})

function GotutorDiag()
  local n = #vim.diagnostic.get(0, { severity = vim.diagnostic.severity.ERROR })
  return n > 0 and ('✗ ' .. n) or ''
end

-- go test / run / vet into the quickfix list, from the current file's package.
local function go(args)
  return function()
    vim.cmd('silent! wall')
    vim.cmd('compiler go')
    vim.opt_local.makeprg = 'go -C ' .. vim.fn.fnameescape(vim.fn.expand('%:p:h')) .. ' ' .. args
    vim.cmd('make!')
    vim.cmd('cwindow')
  end
end
vim.keymap.set('n', '<leader>t', function()
  vim.cmd('silent! wall')
  vim.cmd('botright split | resize 12 | terminal cd ' .. vim.fn.shellescape(vim.fn.expand('%:p:h'))
    .. ' && go test -race -count=1 -timeout 20s -v .')
end, { desc = 'go test' })
vim.keymap.set('n', '<leader>r', function()
  vim.cmd('silent! wall')
  vim.cmd('botright split | resize 12 | terminal cd ' .. vim.fn.shellescape(vim.fn.expand('%:p:h')) .. ' && go run .')
end, { desc = 'go run' })
vim.keymap.set('n', '<leader>v', go('vet .'), { desc = 'go vet' })
vim.keymap.set('n', '<leader>n', '/TODO(you)<CR>', { desc = 'next TODO' })
vim.keymap.set('t', '<Esc><Esc>', [[<C-\><C-n>]])
