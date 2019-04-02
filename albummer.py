import sys
import os
import markdown
import base64
from tqdm import tqdm


img_extensions = ['.jpg', '.jpeg', '.png']
vid_extensions = ['.mpg', '.mpeg', '.mp4']


def help():
    print(f"""Usage: {sys.argv[0]} command options [global flags]
Where command can be:
  make-template media_folder output.alb [num_cols] [order] [custom.css]
    This will create the album file, ready for editing, as the first step 
    of creating an HTML album.

    Arguments:
    - media_folder : the folder containing images and videos
    - output.alb   : the album file to be generated
    - num_cols     : optional, default=3. The number of columns to use when 
                     laying out images.  Videos will always be placed on a 
                     separate line.
    - order        : optional, default=asc : Sort order of the media, by file 
                     timestamp. If you specify anything other than asc, tben 
                     descending order (newest first) will be used.
    - custom.css   : optional, default=default.css : for pros: specify your 
                     custom CSS file
   
  generate album_file
    Generates the single-file HTML from an album file, with extension .html

    Arguments:
    - album_file   : the album file to be converted. If album_file is 
                     my_fotos.alb, the generated HTML file will be named 
                     my_fotos.html
Global Flags:
  -v               : If you pass in -v, verbose output will be displayed 
""")
    sys.exit(0)


def main():
    verbose = False

    if len(sys.argv) == 1:
        help()

    argv = []
    for arg in sys.argv:
        if arg == '-v':
            verbose = True
            continue
        argv.append(arg)

    cmd = argv[1]
    argv = argv[2:]

    if cmd == 'make-template':
        make_template(argv, verbose)
    elif cmd == 'generate':
        generate(argv, verbose)
    else:
        help()


def abort(msg, exit_code=1):
    print(msg)
    sys.exit(exit_code)

def log(msg):
    print(msg)

def make_template(argv, verbose):
    def get_next_n_from_list(n, l):
        i = 0
        ret = []
        while l and i < n:
            ret.append(l.pop())
            i += 1
        return ret

    if len(argv) < 1:
        abort('Please specify a media folder and an output file name')
    elif len(argv) < 2:
        abort('Please specify an output file name')
    
    folder = argv[0]
    outfile = argv[1]
    css = os.path.join(os.path.dirname(os.path.abspath(sys.argv[0])), 
            'default.css')
    num_cols = 3
    order = 'asc'
    if len(argv) > 2:
        num_cols = int(argv[2])
    if len(argv) > 3:
        order = argv[3]
    if len(argv) > 4:
        css = argv[4]
    
    all_media = []
    for f in os.listdir(folder):
        _, ext = os.path.splitext(f)
        if ext in img_extensions or ext in vid_extensions:
            f = os.path.join(folder, f)
            if not os.path.isfile(f):
                continue
            all_media.append(f)
    # list needs to be reversed for asc, since we are going to pop
    reverse = True    
    if order != 'asc':
        reverse = False
    all_media.sort(key=os.path.getmtime, reverse=reverse)
    all_media = [os.path.basename(f) for f in all_media]

    # prepare media lines, video files will always be put on a single line
    media_body = ''
    line_len = 0
    while all_media:
        m = all_media.pop()
        _, ext = os.path.splitext(m)
        if ext in vid_extensions:
            # make a new line
            if line_len > 0:
                media_body += '\n'
            media_body += f'\n{m}\n\n'
            line_len = 0
        else:
            if line_len > 0:
                media_body += '   '
            media_body += m
            line_len += 1
            if line_len == num_cols:
                media_body += '\n'
                line_len = 0

    # create template
    title = os.path.basename(folder)

    with open(outfile, 'wt') as f:
        f.write(f""":folder {folder}
:show_filenames
:use {css}

# {title}

{media_body}
""")
    print(f'Generated {outfile}')


def generate(argv, verbose):
    if len(argv) < 1:
        abort('Please specify input file!')

    with open(argv[0], 'rt') as f:
        lines = [l.strip() for l in f.readlines()]
    
    lines.reverse()

    folder = None
    show_filenames = False
    css = 'default.css'
    all_media = []

    html_bodies = []
    html_head = ''

    lc = 0

    print(f'The Albummer is processing {argv[0]}')
    tq = tqdm(lines, bar_format='    {l_bar}{bar}|   ', ascii=True)

    while lines:
        line = lines.pop()
        lc += 1

        try:
            if not line:
                tq.update()
                continue
            if line.startswith(':'):
                cols = line.split()
                if cols[0] == ':folder':
                    folder = cols[1]
                    all_media = []
                    if verbose:
                        tq.write(f'{lc:3d} CONTROL   : {line}')
                    for f in os.listdir(folder):
                        _, ext = os.path.splitext(f)
                        if ext in img_extensions or ext in vid_extensions:
                            f = os.path.join(folder, f)
                            if not os.path.isfile(f):
                                continue
                            all_media.append(os.path.basename(f))
                    tq.update()
                elif cols[0] == ':show_filenames':
                    tq.update()
                    if verbose:
                        tq.write(f'{lc:3d} CONTROL   : {line}')
                    show_filenames = True
                elif cols[0] == ':use':
                    tq.update()
                    if verbose:
                        tq.write(f'{lc:3d} CONTROL   : {line}')
                    css = cols[1]
                    with open(css, 'rt') as f:
                        css = f.read()
                        html_head = f'<style>{css}</style>'
            else:
                cols = line.split()
                if cols[0] in all_media:
                    # we have a media line
                    if verbose:
                        tq.write(f'{lc:3d} MEDIA     : {line}')
                    num_cols = len(cols)
                    percent = int(100 / num_cols)
                    html = '<div align="center"><table><tr>'
                    for col in cols:
                        html += f'<td style="width:{percent}%;">' 
                        _, ext = os.path.splitext(col)
                        if ext in img_extensions:
                            data = img_to_html(folder, col)
                        else:
                            data = vid_to_html(folder, col)
                        html += data
                        html += '</td><td width="10px"></td>'
                    html += '</tr></table></div>\n'
                    html_bodies.append(html)
                    tq.update()
                else:
                    # we have a markdown block beginning
                    markdown_lines = [(lc, line)]
                    while lines:
                        line = lines.pop()
                        lc += 1
                        if not line:
                            markdown_lines.append((lc, line))
                            continue
                        cols = line.split()
                        if cols[0] in all_media:
                            # we have a media line -> end of markdown, 
                            # put it back
                            lc -= 1
                            lines.append(line)
                            break
                        markdown_lines.append((lc, line))
                    if verbose:
                        tq.write('--')
                        for l, line in markdown_lines:
                            tq.write(f'{l:3d} MARKDOWN  : {line}')
                        tq.write('--')
                    
                    md = '\n'.join([x[1] for x in markdown_lines])
                    tq.update(len(markdown_lines))
                    html = markdown.markdown(md)
                    html_bodies.append(html + '\n')
        except Exception as e:
            abort(f'Line {lc} : {e}')
            
    tq.close()
    ofile = argv[0]
    ofile, ext = os.path.splitext(ofile)
    ofile += '.html'
    print(f'Writing {ofile}...')
    with open(ofile, 'wt') as f:
        f.write(f'<!DOCTYPE html><html><head>{html_head}</head>\n<body>')
        for html_body in tqdm(html_bodies,
                              bar_format='    {l_bar}{bar}|   ',
                              ascii=True):
            f.write(f'{html_body}')
        f.write(f'</body>\n</html>')
    print('READY.')


def img_to_html(folder, img):
    f = os.path.join(folder, img)
    if img.lower().endswith('.png'):
        imgformat = 'png'
    else:
        imgformat = 'jpeg'
    data = base64.b64encode(open(f, 'rb').read())
    data = data.decode('utf-8').replace('\n', '')  
    return f'''<img width="100%" 
                    src="data:image/{imgformat};base64,{data}">
               </img>'''


def vid_to_html(folder, vid):
    f = os.path.join(folder, vid)
    data = base64.b64encode(open(f, 'rb').read())
    data = data.decode('utf-8').replace('\n', '')  
    return f'''<video width="100%" 
                      controls 
                      src="data:video/mp4;base64,{data}">
               </video>'''


if __name__ == '__main__':
    main()
