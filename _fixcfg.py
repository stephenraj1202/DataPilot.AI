with open('config.ini', 'r') as f:
    lines = f.readlines()

# Remove stray bare $ lines (artifacts from bat file escaping)
cleaned = []
for l in lines:
    stripped = l.strip()
    if stripped in ['$', '$$', '$$$']:
        continue
    cleaned.append(l)

with open('config.ini', 'w') as f:
    f.writelines(cleaned)

print('config.ini cleaned.')
for l in cleaned[:10]:
    print(repr(l), end='')
