-- gloss.lua: Rex actions for gloss, written by `gloss skill install rex`.
--
-- Load it from init.lua with require("gloss"), then run `rex config reload`.
-- It defines three actions and one key:
--
--   grex [direction=DIR] [file=PATH] [args=...]  open gloss in a split
--                                                (right, left, above, below)
--   gloss_companion [file=PATH] [label=NAME]     toggle a gloss split beside the
--                                                block at hand that follows its
--                                                shell and editor (cmd+shift+g)
--   gloss_push file=PATH [page=N] [label=NAME]   show a file in that split
--
-- The companion and push actions speak to the viewer over its Unix socket
-- (gloss --socket / --remote), never by typing into it. gloss-companion.lua,
-- beside this file, is the script that keeps the split in step; the toggle
-- starts it detached. grex, also installed by `gloss skill install rex`, wraps
-- `rex split` for the shell: `grex [right|left|above|below] [GLOSS_ARGS...]`.
-- The gloss binary is the login shell's, or GLOSS in Rex's environment.

local LABEL = "gloss-preview"

-- The folder this file is in, where gloss-companion.lua is too. Rex names a
-- chunk by its path, with or without Lua's usual @; a chunk with no folder in
-- its name (-e, stdin) falls back to Rex's config folder.
local function here()
  local source = (debug.getinfo(1, "S").source or ""):gsub("^[@=]", "")
  local dir = source:match("^(.*)[/\\][^/\\]*$")
  if dir and dir ~= "" then
    return dir
  end
  local config = os.getenv("REX_CONFIG")
  if config then
    return config:match("^(.*)[/\\][^/\\]*$") or "."
  end
  local xdg = os.getenv("XDG_CONFIG_HOME")
  if xdg and xdg ~= "" then
    return xdg .. "/rex"
  end
  return (os.getenv("HOME") or ".") .. "/.config/rex"
end

local COMPANION = here() .. "/gloss-companion.lua"

-- socket_name is the viewer's socket for a session and label: letters, digits,
-- dots, dashes and underscores, at most 64 of them, which is what gloss takes.
-- gloss-companion.lua computes the same name; keep the two in step.
local function socket_name(session_id, label)
  local id = (session_id or ""):gsub("[^%w]", "")
  id = id:sub(-12)
  local name = (label or LABEL):gsub("[^%w%._-]", "-"):sub(1, 40)
  return "rex-" .. id .. "-" .. name
end

-- find_preview is the block labeled label in the session, or nil.
local function find_preview(session_id, label)
  local res = rex.call("session.list_blocks", { session_id = session_id })
  for _, b in ipairs(res and res.blocks or {}) do
    if b.label == label and b.placement == "placed" then
      return b.block_id
    end
  end
  return nil
end

-- block_dir is the folder the terminal block last reported, decoded from the
-- file:// URL that OSC 7 carries, or nil.
local function block_dir(block_id)
  if not block_id then
    return nil
  end
  local res = rex.block.call("com.superlogical.terminal", "pwd", { block_id = block_id })
  local pwd = res and res.pwd
  if not pwd or pwd == "" then
    return nil
  end
  pwd = pwd:gsub("^file://[^/]*", "")
  pwd = pwd:gsub("%%(%x%x)", function(hex)
    return string.char(tonumber(hex, 16))
  end)
  return pwd
end

local function trim(s)
  return (tostring(s or ""):gsub("^%s+", ""):gsub("%s+$", ""))
end

local function file_exists(path)
  local f = io.open(path, "r")
  if f then
    f:close()
    return true
  end
  return false
end

-- Rex runs these actions in its server, whose PATH is the system's
-- (/usr/bin:/bin:...), not the shell's: neither gloss nor rex itself is on it.
-- Each is asked of the login shell once, then looked for in the usual places.

-- from_shell is what the login shell finds for `command -v name`, or nil.
local function from_shell(name)
  local shell = os.getenv("SHELL")
  if not shell or shell == "" then
    shell = "/bin/sh"
  end
  local res = rex.exec({ shell, "-lc", "command -v " .. name }, { timeout = 10 })
  local found = res and res.ok and trim(res.stdout) or ""
  if found ~= "" then
    return found
  end
  return nil
end

-- has_socket is whether the gloss at path knows --socket: one from before
-- 0.6, such as a Homebrew install not yet upgraded, does not, and the shell
-- may well find that one ahead of a newer build.
local function has_socket(path)
  local res = rex.exec({ path, "--help" }, { timeout = 10 })
  return res ~= nil and res.ok and res.stdout:find("--socket", 1, true) ~= nil
end

-- gloss is the gloss binary: GLOSS in the server's environment, else the first
-- of the shell's and the usual places that knows --socket, else the shell's.
local gloss_bin = nil
local gloss_old = nil -- A gloss found but too old, to say so.
local function gloss()
  if gloss_bin then
    return gloss_bin
  end
  local named = os.getenv("GLOSS")
  if named and named ~= "" then
    gloss_bin = named
    return gloss_bin
  end
  local home = os.getenv("HOME") or ""
  local candidates = { from_shell("gloss"), home .. "/go/bin/gloss", home .. "/.local/bin/gloss", "/opt/homebrew/bin/gloss", "/usr/local/bin/gloss" }
  for _, path in ipairs(candidates) do
    if path and file_exists(path) then
      if has_socket(path) then
        gloss_bin = path
        return gloss_bin
      end
      gloss_old = gloss_old or path
    end
  end
  gloss_bin = gloss_old or "gloss"
  return gloss_bin
end

-- rex_bin is the rex command: REX_BIN_DIR when the server passes it on, else
-- the shell's, else where the macOS app keeps it, else the name alone.
local rex_cmd = nil
local function rex_bin()
  if rex_cmd then
    return rex_cmd
  end
  local dir = os.getenv("REX_BIN_DIR")
  if dir and dir ~= "" and file_exists(dir .. "/rex") then
    rex_cmd = dir .. "/rex"
    return rex_cmd
  end
  local home = os.getenv("HOME") or ""
  local app = home .. "/Library/Application Support/com.superlogical.rex/RexServer/Versions/Current/Rex Server.app/Contents/MacOS/rex"
  rex_cmd = from_shell("rex") or (file_exists(app) and app) or "rex"
  return rex_cmd
end

-- check_gloss stops with a clear word when the only gloss found is too old.
local function check_gloss()
  local bin = gloss()
  if gloss_old and bin == gloss_old then
    error(bin .. " is a gloss without --socket; install gloss 0.6 or later, or set GLOSS to one in Rex's environment", 0)
  end
  return bin
end

local DIRECTIONS = {
  right = { direction = "horizontal", side = "after" },
  left  = { direction = "horizontal", side = "before" },
  above = { direction = "vertical",   side = "before" },
  below = { direction = "vertical",   side = "after" },
  up    = { direction = "vertical",   side = "before" },
  down  = { direction = "vertical",   side = "after" },
}

rex.action{
  name = "grex",
  title = "gloss: open in a split",
  description = "Open gloss in a split (right, left, above, or below) and pass the arguments to it.",
  category = "gloss",
  keywords = "grex gloss split open preview",
  args = { direction = "string?", file = "string?", page = "integer?", args = "string?" },
  run = function(ctx, args)
    local name = (args.direction or "right"):lower()
    local split_dir = DIRECTIONS[name]
    if not split_dir then
      error("direction must be right, left, above, or below", 0)
    end
    local command = { gloss() }
    if args.args then
      for word in args.args:gmatch("%S+") do
        table.insert(command, word)
      end
    end
    if args.page then
      table.insert(command, "--page")
      table.insert(command, tostring(args.page))
    end
    if args.file then
      table.insert(command, args.file)
    end
    local split, err = rex.call("session.new_split", {
      session_id = ctx.session_id,
      anchor_id = ctx.block_id,
      direction = split_dir.direction,
      side = split_dir.side,
      focus = true,
      layout = rex.layout.block{
        flavor = "com.superlogical.terminal.shell",
        options = {
          command = command,
          cwd = block_dir(ctx.block_id),
          exit = { quick_exit_threshold_ms = 2000 },
        },
      },
    })
    if not split then
      error("opening the gloss split: " .. tostring(err), 0)
    end
    return { status = "opened", direction = name, block_id = split.block_ids and split.block_ids[1], gloss = gloss() }
  end,
}

rex.action{
  name = "gloss_companion",
  title = "gloss: toggle the companion preview",
  description = "Open a gloss split beside this block that follows its shell and editor, or close it.",
  category = "gloss",
  keywords = "gloss preview pager companion",
  args = { file = "string?", page = "integer?", label = "string?" },
  run = function(ctx, args)
    local label = args.label or LABEL
    local block = find_preview(ctx.session_id, label)
    if block then
      local _, err = rex.call("block.close", { session_id = ctx.session_id, block_id = block })
      if err then
        error("closing " .. label .. ": " .. tostring(err), 0)
      end
      return { status = "closed", label = label, block_id = block }
    end
    if not ctx.block_id then
      error("gloss_companion needs a block to sit beside; run it from a terminal", 0)
    end
    local argv = {
      rex_bin(), "do", "-s", ctx.session_id, "-b", ctx.block_id, COMPANION,
      "source_block=" .. ctx.block_id,
      "label=" .. label,
      "socket=" .. socket_name(ctx.session_id, label),
      "gloss=" .. check_gloss(),
    }
    if args.file then
      table.insert(argv, "file=" .. args.file)
    end
    if args.page then
      table.insert(argv, "page=" .. tostring(args.page))
    end
    -- Its output is left alone: discarding it (stdout = false) closes the
    -- script's descriptors when this action returns, and the script dies.
    local handle, err = rex.exec.start(argv, { detach = true, cwd = block_dir(ctx.block_id) })
    if not handle then
      error("starting the companion: " .. tostring(err), 0)
    end
    return { status = "started", label = label, source_block = ctx.block_id, pid = handle.pid, script = COMPANION, gloss = gloss() }
  end,
}

rex.action{
  name = "gloss_push",
  title = "gloss: show a file in the companion preview",
  description = "Show a file in the gloss companion split, opening the split first when there is none.",
  category = "gloss",
  keywords = "gloss preview show open",
  args = { file = "string", page = "integer?", label = "string?" },
  run = function(ctx, args)
    local label = args.label or LABEL
    local argv = { check_gloss(), "--remote", socket_name(ctx.session_id, label), args.file }
    if args.page then
      table.insert(argv, "--page")
      table.insert(argv, tostring(args.page))
    end
    local res = rex.exec(argv, { cwd = block_dir(ctx.block_id), timeout = 40 })
    if res and res.ok then
      return { status = "shown", file = args.file, label = label }
    end
    if find_preview(ctx.session_id, label) then
      -- The split is there but its viewer refused, or is gone: say why.
      error(trim(res and res.stderr ~= "" and res.stderr or "gloss --remote failed"), 0)
    end
    return rex.invoke("gloss_companion", { file = args.file, page = args.page, label = label })
  end,
}

rex.bind("cmd+shift+g", "gloss_companion")
