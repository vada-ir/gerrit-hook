#!/bin/bash
set -euo pipefail

SCRIPT_DIR=$(readlink -f $(dirname ${BASH_SOURCE[0]}))

all=(change-abandoned change-restored comment-added merge-failed project-created ref-updated change-merged cla-signed draft-published hashtags-changed patchset-created ref-update reviewer-added topic-changed)

for i in "${all[@]}"
do
(
cat <<EOFSCRIPT
#!/bin/bash 
echo "NEW==========================" >> $HOME/ghooks/app.log
echo 'called ${i} '"\$@" >>  $HOME/ghooks/app.log
echo "ENV==========================" >> $HOME/ghooks/app.log
env >>  $HOME/ghooks/app.log
echo "OUT==========================" >> $HOME/ghooks/app.log
${SCRIPT_DIR}/ghook --action ${i} "\$@" >> $HOME/ghooks/app.log
echo "EXIT=========================" >> $HOME/ghooks/app.log
echo "exit code was $?" >> $HOME/ghooks/app.log 
echo "DONE========================="  >> $HOME/ghooks/app.log

EOFSCRIPT
) >  ${SCRIPT_DIR}/${i} 
chmod a+x ${SCRIPT_DIR}/${i} 
done
